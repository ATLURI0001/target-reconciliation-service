# Local execution

TargetReconciliationService (TRS) provides an API/PostgreSQL mode and an isolated one-cluster reliability lab. Two-cluster failover, outbound agents, multi-cloud infrastructure, ELK, Argo CD delivery, and a full local Jenkins installation remain separate milestones.

## Prerequisites

Use Linux, macOS, or Windows with WSL2. Install Go at the version in `go.mod`, Git, Python 3, Docker Engine/Desktop with Compose v2, kubectl, and Kind. Helm 3 is required for chart validation. Run `make doctor` for actionable missing-tool checks. The lab requires a working Docker daemon with local Kubernetes support; Linux hosts require cgroup v2 for the pinned Kubernetes 1.35 node image.

A setup estimate is 4 CPU cores and 8 GiB available memory for the one-cluster lab, not a measured minimum. PostgreSQL has a 512 MiB container memory limit. Kind node resource use is controlled by the container runtime; it is not bounded by the supplied node YAML. The sample application uses synthetic HTTP responses and explicitly permits TRS remediation only in `trs-demo`.

## API and database only

```sh
make init
make up
make smoke
```

The generated `.env` has independent random database and API credentials. API 8080 and database5432 are bound to loopback. `TRS_DATABASE_PORT` can override the host database port. The Compose API deliberately has no Kubernetes credentials mounted, so no cluster data appears until observations are collected by a Kubernetes-enabled controller. Optional `make metrics-up` adds Prometheus and Grafana; see `observability/README.md`.

`make down` preserves PostgreSQL data. Development token auth, local plaintext HTTP, and the local database network do not provide production identity or transport security. Do not expose this profile publicly.

## Automated real Kubernetes lab

```sh
make lab-up
make lab-demo
make lab-down
```

`lab-up` first checks its prerequisites and refuses an existing `.lab` directory, an existing named Kind cluster, or existing `trs-lab` Compose resources. It requires free loopback ports 8080,55432, and 18080. It then creates:

- Kind cluster `target-reconciliation-service-lab`, using a digest-pinned Kubernetes 1.35.0 node image.
- A private `.lab/kubeconfig`, without changing the user's default kubeconfig.
- The real `ReliabilityTarget` CRD, `trs-demo` namespace, sample catalog Deployment and Service.
- A dedicated Compose PostgreSQL database in project `trs-lab`, with port 55432 and random temporary credentials.
- The compiled Go API/controller host process, authenticated with those temporary credentials and connected to that dedicated cluster and database.

The host controller runs with its private Kind kubeconfig's lab administrator access. The Helm service-account deployment provides the namespace-scoped RBAC alternative. No unrestricted kubeconfig is distributed to an agent or committed. These scripts do not create cloud resources.

Invoking `make lab-demo` explicitly requests a test-only failed rollout and approval against this lab. The script checks the exact local context, derives a recovery image digest from the actual running pod, creates a target, and waits for a current-generation `Ready=True` condition. It verifies real service HTTP before introducing an invalid image. The catalog's `maxSurge:0` and `maxUnavailable:1` strategy deliberately permits a real interruption so an old healthy pod cannot mask the failure. This is a destructive test setting confined to the sample application.

The demo waits for `Ready=False`, an actual `AwaitingApproval` incident and a matching unhealthy image. It proves a viewer gets HTTP403 and the actual Deployment image remains unchanged. The operator then approves recovery. Success requires the incident to be `Resolved`, a current-generation target `Ready=True`, successful Kubernetes rollout, exact restoration of the recorded digest, and a real HTTP response from the service. A failed condition returns a nonzero exit code. The original policy is restored on exit. Temporary service port-forward processes are stopped.

## Evidence and cleanup

Successful and failed executions save logs and collected objects under `evidence/lab-local/`, or `TRS_EVIDENCE_DIR` if set. Evidence includes the command log, before/after service responses, target conditions and observed generations, real persisted workflow snapshots, policy denial check, audit timeline, metrics, and final workload state. It excludes `.lab/`, credentials, request headers, and kubeconfig. No expected result is written as a successful measurement before the associated check passes.

`make lab-down` sends SIGTERM only to the recorded controller when its command matches the expected binary, deletes only the lab-created named Kind cluster, stops the dedicated Compose project, and intentionally deletes its disposable database volume. Temporary credentials and kubeconfig are removed. Failed startup also attempts this scoped cleanup. If a cleanup tool fails, inspect its report before retrying; no unrelated project is deleted. Evidence remains for diagnosis.

This execution environment could not start Docker: the actual rootless attempt failed during UID/GID setup because `newuidmap` was unavailable, and cgroup mounting was also denied. See `evidence/trs-docker-attempt.txt`. Therefore, the full Docker/Kind/service-HTTP demo has not been measured here. The GitHub Actions Kind job contains the actual commands for a Docker-capable hosted runner, but a supplied workflow is not proof that hosted execution passed.

## Helm installation boundary

The chart packages the CRD on first installation. Helm does not automatically upgrade CRDs, so run `kubectl apply -f config/crd/` explicitly before upgrading an existing lab. The chart also expects `trs-demo` to exist, a reachable PostgreSQL database, and a private existing Secret in the release namespace named `target-reconciliation-service-runtime` containing `DATABASE_URL`, `TRS_OPERATOR_TOKEN`, and `TRS_VIEWER_TOKEN`.

Do not copy a host URL using `127.0.0.1` into a pod: it would point at that pod, not the host. Database installation, transport security, and credentials are explicit chart prerequisites.

```sh
make helm
docker build -t target-reconciliation-service:local .
kind load docker-image target-reconciliation-service:local --name target-reconciliation-service-lab
# After creating the namespace, reachable database, and private credential Secret:
helm upgrade --install target-reconciliation-service deploy/helm/target-reconciliation-service \
  --namespace target-reconciliation-service --create-namespace
kubectl -n target-reconciliation-service rollout status deployment/target-reconciliation-service --timeout=120s
```

The chart limits controller RBAC to `trs-demo` deployments, targets/status, and leader-election leases. The management ingress NetworkPolicy permits labeled API clients; enforcement requires a compatible CNI and is not provided by default Kind networking. Egress restrictions and production identity remain incomplete.
