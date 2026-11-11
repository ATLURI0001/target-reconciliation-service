# Kubernetes bridge

The first bridge runs inside the management process, using a namespace-scoped
controller-runtime cache. It watches `ReliabilityTarget` resources and polls
Deployment observations every five seconds; both target and Deployment reads
use the manager's **uncached APIReader**. The cache supplies watch events only;
API connectivity loss cannot restamp cached state as fresh. Failed live reads
produce no observation or execution claim. Errors enter controller-runtime's
rate-limited retry queue. It deliberately supports only `rp-demo`.

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
`rp-demo` Role therefore is an explicit trust boundary. Restrict who can create
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
