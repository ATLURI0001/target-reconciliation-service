# Reproducible scenarios and evidence boundaries

## B/C/E: failure detection, authorized rollback, rejected authorization

Status: controller/service/persistence behavior verified against an actual Kubernetes API server, etcd and PostgreSQL; synthetic rollout status supplied by the integration tests. Live container recovery remains unverified here.

Prerequisites: Docker, Kind, kubectl, Go 1.27.4+, Python3, curl; dedicated context `kind-resilienceplane-lab`; API and controller running as described in local-lab.md. Apply examples/demo/deployment.yaml and wait until the catalog workload is actually Ready. The supplied image tag bootstraps the healthy workload; the script captures its actual running pullable digest for the recovery target. This does not certify image provenance.

In another terminal:

```sh
RP_DEMO_EXECUTE=yes ./scripts/demo.sh
```

The explicit opt-in script refuses other contexts, creates a ReliabilityTarget for rp-demo/catalog, enables the narrowly scoped test policy, sets a deliberately nonexistent catalog image, waits for a genuine incident, asserts viewer approval returns403, approves as operator, and polls for Resolved. Expected success requires a matching execution annotation/image and actual Deployment rollout counters. Any Failed/Cancelled state or deadline ends nonzero. Inspect `/v1/audit`, `kubectl -n rp-demo get deploy catalog -o yaml`, and console for evidence. The API observation endpoint does not exist and the script cannot fabricate verification.

Failure conditions: missing tools/context; registry unreachable; runtime imageID is not a pullable digest; initial rollout not healthy; stale observations; policy refusal; image pull failure; controller down; verification timeout. The script does not silently claim these outcomes succeeded. It uses default cluster ID local and refuses a non-loopback API. Reusing the same persisted target after a previous failed terminal episode may require an observed healthy baseline and a deliberate new experiment.

Abort: cancel the incident with rpctl and set emergencyStop=true in a valid full policy. Cancellation cannot retract an already-sent patch. Stop generating faults, inspect the Deployment, and deliberately restore the known-good digest if required. The script leaves test policy and evidence available for inspection; reset dryRun=true afterward. Teardown uses only project-managed resources in local-lab.md.

For the actually executed control-plane test:

```sh
make envtest
# Add isolated PostgreSQL for the full durable variant:
TEST_DATABASE_URL='your-isolated-database-url' make envtest
```

See evidence/reviewed-integration.txt. Envtest is not a replacement for the above live Kind demonstration.

## A: GitHub-to-Kubernetes delivery

Status: planned end-to-end. ci/Jenkinsfile validates/scans/builds/generates SBOM when installed on a suitable agent. Publication, GitOps commit, Argo CD sync, canary and rollback pipeline are not implemented or demonstrated. A future test must pin image digests, preserve provenance and record rollout health. No remote repository or image publication was performed.

## D: two-cluster stateless interruption

Status: planned. No route-switch executor, secondary cluster enrollment, fencing or failover is implemented. The acceptance test must verify stale-health refusal, exclusive serving ownership, preconfigured healthy fallback, route change, abort and observed request recovery. No stateful/cross-region consistency claim may be inferred from the initial controller.
