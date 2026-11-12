# Kubernetes bridge

The first bridge runs inside the management process, using a namespace-scoped
controller-runtime cache. It watches `ReliabilityTarget` resources and polls
Deployment observations every five seconds; both target and Deployment reads
use the manager's **uncached APIReader**. The cache supplies watch events only;
API connectivity loss cannot restamp cached state as fresh. Failed live reads
produce no observation or execution claim. Errors enter controller-runtime's
rate-limited retry queue. It deliberately supports only `trs-demo`.

A target names a Deployment, container and a **known-good digest-pinned image**.
The reference demo script resolves the actual image digest from running Pods,
then applies its target before inducing a bad image release. This is a rollback
of a failed release, adding value beyond Kubernetes' ordinary Pod replacement.
It does not claim application diagnosis from Deployment availability.

Policy must authorize the explicit plan. Before a patch, the controller checks
namespace, immutable Deployment UID, opt-in, GitOps ownership and image. Atomic
JSON Patch tests resourceVersion, UID and original image. An execution marker
in the Pod template makes replay idempotent following an uncertain response.
A generation, replica count or image change rejects the approved plan. A
status-only resourceVersion change is permitted after a fresh read; the atomic
patch tests that current resourceVersion to detect a concurrent write. API
errors, including uncertain writes, retain the same execution key and retry
until the workflow deadline. The execution marker is checked before generation
validation because our own successful image change increases generation.

No Kubernetes mutation endpoint is exposed to external agents. The controller
uses its local scoped credential; an outbound authenticated agent remains
planned. Role resource scope cannot constrain patch to a single label; the
`trs-demo` Role therefore is an explicit trust boundary. Restrict who can create
ReliabilityTarget resources and modify the namespace. Argo/Flux owners are
rejected; arbitrary unlabelled external owners cannot be reliably detected.

`target_test.go` uses controller-runtime's fake client and validates local logic,
not API-server JSON Patch atomicity or an actual rollout. The envtest test skips
unless `KUBEBUILDER_ASSETS` points to matching kube-apiserver/etcd binaries.

## Real API integration

```sh
KUBEBUILDER_ASSETS="$(go run sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.0.0-20260305142021-f9589b9f2b9d use -p path 1.35.0)"
export KUBEBUILDER_ASSETS
# Optional: point at a disposable PostgreSQL database whose test user may create schemas.
# export TEST_DATABASE_URL='postgres://USER:PASSWORD@localhost:5432/DATABASE?sslmode=disable'
go test -count=1 -v ./controllers -run TestEnvtestAtomicPatch
```

The test launches real kube-apiserver and etcd processes bound to loopback. It
checks the CRD schema, API-server JSON Patch preconditions, and namespace RBAC
using service-account impersonation. It runs detection, denied viewer approval,
operator approval, image mutation and verification through the real controller
and core service. A PostgreSQL variant runs only with `TEST_DATABASE_URL`, uses
an isolated test schema, and closes/reopens the database pool before final
verification. Schema cleanup drops only that test-created schema.

**Status is explicitly supplied by the test.** Envtest does not run a scheduler,
kubelet or Deployment controller. These tests establish API/storage integration,
not a successful Pod rollout, application service recovery, or measured RTO.
`evidence/kubernetes-postgres-envtest.txt` records the actual successful run using
Kubernetes 1.35.0, etcd 3.6.6 and PostgreSQL 17.11. No Kind runtime was available.

## ReliabilityTarget status contract

`ReliabilityTarget` exposes a real Kubernetes `/status` subresource. Its
`status.observedGeneration` and each condition's `observedGeneration` identify
the **CR generation evaluated**, including an invalid spec or failed attempt.
They are distinct from the referenced Deployment's generation.

| Condition | Meaning |
| --- | --- |
| `Ready=True` | The uncached Deployment observation satisfies the strict rollout predicate: observed generation caught up, total and updated replicas equal desired, and desired replicas are available. |
| `Ready=False` | The Deployment is known absent or the fresh Deployment observation does not satisfy that predicate. |
| `Ready=Unknown` | The live observation, specification or workflow cannot establish readiness, or a recovery action awaits a fresh verification observation. |
| `Reconciled=True` | This controller pass recorded its observation and evaluated the workflow, or submitted the authorized action for verification. This is not a statement that remediation was authorized or recovery completed. |
| `Reconciled=False` | Observation, spec validation, workflow persistence/evaluation or execution failed or was rejected. |

Reasons include `DeploymentReady`, `RolloutIncomplete`,
`DeploymentNotFound`, `ObservationUnavailable`, `SpecInvalid`, `WorkflowFailed`,
`ExecutionRejected`, `ExecutionUncertain` and `VerificationPending`.
`lastTransitionTime` changes only when a condition's True/False/Unknown status
changes; a reason, message or generation change retains that time.

`status.lastObservation` retains the last successfully read Deployment evidence,
including its UID, generation, replica counts and image. It deliberately has no
per-poll timestamp; backend observation timestamps remain the source for data
freshness. Unknown observations preserve this historical evidence. During a
complete Kubernetes API outage, publishing Unknown may itself fail; existing
status then remains historical and must not be treated as a live heartbeat.

Status patches reread the CR without the cache, fence its UID and generation,
and use optimistic resourceVersion locking with conflict retries. Unchanged
status causes no patch. Status write errors preserve the original reconciliation
error. Watches filter status-only changes with `GenerationChangedPredicate`;
regular successful reconciliation still schedules a five-second poll.

Inspect the result with:

```sh
kubectl -n trs-demo get reliabilitytargets
kubectl -n trs-demo get reliabilitytarget catalog -o yaml
```

Consumers must check that the Ready condition's `observedGeneration` equals
`metadata.generation` before interpreting it as the current desired spec.
A Deployment-only status update never increments the CR generation. A CR
status-only write also leaves CR `metadata.generation` unchanged; the real API
integration tests verify both the subresource boundary and generation behavior.
