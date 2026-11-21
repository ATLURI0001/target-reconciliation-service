package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	ErrSelfDependency = errors.New("node cannot depend on itself")
	ErrEmptyGraph     = errors.New("dependency graph contains no nodes")
	ErrNodeNotFound   = errors.New("node does not exist in graph")
)

// DependencyGraph manages directed dependencies between reconciliation resources.
// If node A depends on node B, then B must complete reconciliation before A begins.
type DependencyGraph struct {
	mu           sync.RWMutex
	nodes        map[string]struct{}
	dependencies map[string]map[string]struct{} // node -> set of prerequisites
	dependents   map[string]map[string]struct{} // node -> set of downstream dependers
}

// NewDependencyGraph constructs an empty, concurrency-safe dependency graph.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes:        make(map[string]struct{}),
		dependencies: make(map[string]map[string]struct{}),
		dependents:   make(map[string]map[string]struct{}),
	}
}

// AddNode registers a unique resource in the reconciliation graph.
func (g *DependencyGraph) AddNode(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("node identifier cannot be empty")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.ensureNodeUnlocked(id)
	return nil
}

func (g *DependencyGraph) ensureNodeUnlocked(id string) {
	if _, exists := g.nodes[id]; !exists {
		g.nodes[id] = struct{}{}
		g.dependencies[id] = make(map[string]struct{})
		g.dependents[id] = make(map[string]struct{})
	}
}

// AddDependency declares that nodeID depends on dependsOnID (dependsOnID must execute before nodeID).
func (g *DependencyGraph) AddDependency(nodeID, dependsOnID string) error {
	nodeID = strings.TrimSpace(nodeID)
	dependsOnID = strings.TrimSpace(dependsOnID)

	if nodeID == "" || dependsOnID == "" {
		return errors.New("node and dependency identifiers cannot be empty")
	}
	if nodeID == dependsOnID {
		return fmt.Errorf("%w: %s -> %s", ErrSelfDependency, nodeID, dependsOnID)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.ensureNodeUnlocked(nodeID)
	g.ensureNodeUnlocked(dependsOnID)

	g.dependencies[nodeID][dependsOnID] = struct{}{}
	g.dependents[dependsOnID][nodeID] = struct{}{}
	return nil
}

// NodeCount returns the total number of registered nodes in the graph.
func (g *DependencyGraph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// TopologicalSort returns a valid sequential execution order of all nodes.
// Returns ErrCyclicDependency if one or more dependency cycles exist.
func (g *DependencyGraph) TopologicalSort() ([]string, error) {
	waves, err := g.ComputeExecutionWaves()
	if err != nil {
		return nil, err
	}

	totalNodes := 0
	for _, wave := range waves {
		totalNodes += len(wave)
	}

	ordered := make([]string, 0, totalNodes)
	for _, wave := range waves {
		ordered = append(ordered, wave...)
	}
	return ordered, nil
}

// ComputeExecutionWaves partitions the graph into sequential waves of independent nodes.
// All nodes within a single wave have their prerequisites satisfied and can be reconciled in parallel.
func (g *DependencyGraph) ComputeExecutionWaves() ([][]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if len(g.nodes) == 0 {
		return nil, ErrEmptyGraph
	}

	// Calculate in-degree (number of unsatisfied upstream prerequisites)
	inDegree := make(map[string]int, len(g.nodes))
	for n := range g.nodes {
		inDegree[n] = len(g.dependencies[n])
	}

	var waves [][]string
	processed := 0

	for {
		var currentWave []string
		for node, deg := range inDegree {
			if deg == 0 {
				currentWave = append(currentWave, node)
			}
		}

		if len(currentWave) == 0 {
			break
		}

		sort.Strings(currentWave)
		waves = append(waves, currentWave)
		processed += len(currentWave)

		// Remove scheduled nodes and decrement in-degrees for dependent nodes
		for _, node := range currentWave {
			delete(inDegree, node)
			for dependent := range g.dependents[node] {
				if _, ok := inDegree[dependent]; ok {
					inDegree[dependent]--
				}
			}
		}
	}

	if processed < len(g.nodes) {
		cycleNodes := make([]string, 0, len(inDegree))
		for node := range inDegree {
			cycleNodes = append(cycleNodes, node)
		}
		sort.Strings(cycleNodes)
		return nil, fmt.Errorf("%w: unresolved nodes [%s]", ErrCyclicDependency, strings.Join(cycleNodes, ", "))
	}

	return waves, nil
}

// GetDependencies returns the list of prerequisite node IDs that must precede id.
func (g *DependencyGraph) GetDependencies(id string) ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	deps, exists := g.dependencies[id]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, id)
	}

	res := make([]string, 0, len(deps))
	for d := range deps {
		res = append(res, d)
	}
	sort.Strings(res)
	return res, nil
}

// GetDependents returns the list of downstream node IDs waiting on id.
func (g *DependencyGraph) GetDependents(id string) ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	deps, exists := g.dependents[id]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, id)
	}

	res := make([]string, 0, len(deps))
	for d := range deps {
		res = append(res, d)
	}
	sort.Strings(res)
	return res, nil
}
