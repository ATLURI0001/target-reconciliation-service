package core

import (
	"errors"
	"fmt"
	"sort"
)

var (
	// ErrCyclicDependency is returned when a circular dependency is detected in the resource graph.
	ErrCyclicDependency = errors.New("cyclic dependency detected in resource DAG")
	// ErrMissingDependency is returned when a declared dependency does not exist in the resource set.
	ErrMissingDependency = errors.New("declared dependency does not exist in resource set")
	// ErrDuplicateResource is returned when multiple resources share the same ID.
	ErrDuplicateResource = errors.New("duplicate resource ID detected")
)

// ResourceSpec models a stateful infrastructure or database resource entity.
type ResourceSpec struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Version     int64    `json:"version"`
	PayloadHash string   `json:"payload_hash"`
	DependsOn   []string `json:"depends_on"`
}

// ReconciliationStage represents a parallelizable execution tier in the reconciliation plan.
type ReconciliationStage struct {
	StageIndex  int      `json:"stage_index"`
	ResourceIDs []string `json:"resource_ids"`
}

// ExecutionPlan outlines an ordered sequence of stages to safely converge target states.
type ExecutionPlan struct {
	Stages         []ReconciliationStage `json:"stages"`
	TotalResources int                   `json:"total_resources"`
}

// ThreeWayDiff computes state transitions by reconciling desired state against observed actual state
// and an optional base common ancestor.
func ThreeWayDiff(base, actual, desired *ResourceSpec) (ResourceDiff, error) {
	if desired == nil && actual == nil {
		return ResourceDiff{Action: "NOOP", Details: "resource absent in both desired and actual states"}, nil
	}

	if desired == nil && actual != nil {
		return ResourceDiff{
			ResourceID: actual.ID,
			Action:     "DELETE",
			Details:    fmt.Sprintf("resource %s exists in actual state but missing from desired manifest", actual.ID),
		}, nil
	}

	if desired != nil && actual == nil {
		return ResourceDiff{
			ResourceID: desired.ID,
			Action:     "CREATE",
			Details:    fmt.Sprintf("resource %s must be provisioned", desired.ID),
		}, nil
	}

	// Both desired and actual exist
	if desired.PayloadHash == actual.PayloadHash {
		return ResourceDiff{
			ResourceID: desired.ID,
			Action:     "NOOP",
			Details:    "payload hashes match; resource is converged",
		}, nil
	}

	// Conflict detection: if base is present and both actual and desired diverged from base
	if base != nil && base.PayloadHash != actual.PayloadHash && base.PayloadHash != desired.PayloadHash {
		return ResourceDiff{
			ResourceID: desired.ID,
			Action:     "CONFLICT",
			Details:    fmt.Sprintf("three-way divergence detected on %s: actual and desired modified independently from base", desired.ID),
		}, nil
	}

	return ResourceDiff{
		ResourceID: desired.ID,
		Action:     "UPDATE",
		Details:    fmt.Sprintf("payload hash diverged (actual=%s, desired=%s)", actual.PayloadHash, desired.PayloadHash),
	}, nil
}

// PlanExecutionDAG resolves dependencies among resources using Kahn's topological sort algorithm,
// segmenting independent resources into parallelizable execution stages.
func PlanExecutionDAG(specs []ResourceSpec) (*ExecutionPlan, error) {
	if len(specs) == 0 {
		return &ExecutionPlan{Stages: nil, TotalResources: 0}, nil
	}

	nodeMap := make(map[string]ResourceSpec, len(specs))
	inDegree := make(map[string]int, len(specs))
	dependents := make(map[string][]string, len(specs)) // parent -> children

	for _, s := range specs {
		if _, exists := nodeMap[s.ID]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateResource, s.ID)
		}
		nodeMap[s.ID] = s
		inDegree[s.ID] = 0
	}

	// Build adjacency list and in-degree counts
	for _, s := range specs {
		for _, dep := range s.DependsOn {
			if _, exists := nodeMap[dep]; !exists {
				return nil, fmt.Errorf("%w: resource %s depends on missing %s", ErrMissingDependency, s.ID, dep)
			}
			dependents[dep] = append(dependents[dep], s.ID)
			inDegree[s.ID]++
		}
	}

	var stages []ReconciliationStage
	resolvedCount := 0
	stageIndex := 0

	for {
		// Collect all nodes with in-degree 0
		var currentBatch []string
		for id, deg := range inDegree {
			if deg == 0 {
				currentBatch = append(currentBatch, id)
			}
		}

		if len(currentBatch) == 0 {
			break
		}

		// Sort IDs deterministically
		sort.Strings(currentBatch)

		// Remove scheduled nodes from inDegree map so they aren't re-evaluated
		for _, id := range currentBatch {
			delete(inDegree, id)
		}

		stages = append(stages, ReconciliationStage{
			StageIndex:  stageIndex,
			ResourceIDs: currentBatch,
		})
		stageIndex++
		resolvedCount += len(currentBatch)

		// Decrement in-degree for downstream dependents
		for _, parentID := range currentBatch {
			for _, childID := range dependents[parentID] {
				if _, active := inDegree[childID]; active {
					inDegree[childID]--
				}
			}
		}
	}

	if resolvedCount < len(specs) {
		return nil, ErrCyclicDependency
	}

	return &ExecutionPlan{
		Stages:         stages,
		TotalResources: len(specs),
	}, nil
}
