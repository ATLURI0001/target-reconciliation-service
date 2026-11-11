# Local execution profiles

The first slice has two supported startup modes. A two-cluster full lab, outbound agents, multi-cloud provisioning, ELK, Argo CD, and Jenkins installation are later milestones. Their absence is not silently replaced by simulation.

## Prerequisites

Linux, macOS, or Windows with WSL2; Go 1.27.4 or newer; Git; Python 3; Docker Engine/Desktop with Compose v2. Kubernetes mode additionally needs kubectl, Kind, and access to pull the pinned demonstration image. Helm 3 is needed only for chart validation or chart deployment. `make doctor` reports missing tools and installation references.

Suggested initial hardware is 4 CPU cores and 8 GiB available memory for a one-cluster lab. This is a setup estimate, not a measured resource requirement. Compose limits PostgreSQL to 512 MiB and the API to 256 MiB. Kind node memory is controlled by the container runtime and is not bounded by the supplied cluster YAML.

## API and PostgreSQL

```sh
make init
make up
make smoke
```

The private `.env` contains independently generated development tokens and a database password. Keep it out of Git. API and database ports bind only to loopback. `make down` stops project containers but preserves the database volume. No Kubernetes access is mounted into the Compose API.

Do not expose development tokens publicly. PostgreSQL and API traffic in this local profile is plaintext on a local Docker network. Production identity, database TLS, API TLS, and credential rotation are not implemented by this profile.

## One-cluster reliability lab

Run in a dedicated lab context. These commands create only a named local Kind cluster and project-managed namespaces:

```sh
make init
make kind-up
kubectl config current-context
# Expected: kind-resilienceplane-lab
kubectl apply -f config/crd/
kubectl apply -f examples/demo/
docker compose up -d postgres
make build
set -a
. ./.env
set +a
export DATABASE_URL="postgres://resilienceplane:${RP_DATABASE_PASSWORD}@127.0.0.1:5432/resilienceplane?sslmode=disable"
export RP_KUBERNETES=true
export RP_CLUSTER_ID=local
export RP_NAMESPACE=rp-demo
./bin/resilienceplane
```

The host process uses the current kubeconfig and leader election within `rp-demo`. The initial implementation refuses namespaces other than `rp-demo`. The target created by the demo script captures an actually healthy runtime image digest; the control plane does not infer a trustworthy rollback image from a mutable tag. Follow `scripts/demo.sh` and the scenario documentation for policy, approval, rollout fault, and verification operations. Dry-run is the default and must be explicitly disabled for the permitted lab policy before an actual change.

The host process runs with the kubeconfig user's privileges. For least-privilege deployment, use the chart service account in a dedicated management namespace. The chart limits its application Role to `rp-demo` and leases to `rp-demo`; it does not install a cluster-admin role.

## Helm deployment boundary

The chart packages the ReliabilityTarget CRD in `crds/` and installs it on first Helm installation. Helm does not automatically upgrade CRDs; apply `config/crd/` explicitly for schema upgrades. The `rp-demo` namespace must exist before installation. It also expects a reachable PostgreSQL database and an existing Secret named `resilienceplane-runtime` in the release namespace with keys `DATABASE_URL`, `RP_OPERATOR_TOKEN`, and `RP_VIEWER_TOKEN`. Create that Secret from a private local file; never commit a Secret manifest or pass credentials in a recorded shell command.

```sh
make helm
# Build/load the control-plane image in the named Kind cluster:
docker build -t resilienceplane:local .
kind load docker-image resilienceplane:local --name resilienceplane-lab
# After separately creating a reachable database and private Secret:
helm upgrade --install resilienceplane deploy/helm/resilienceplane \
  --namespace resilienceplane --create-namespace
kubectl -n resilienceplane rollout status deployment/resilienceplane --timeout=120s
kubectl -n resilienceplane port-forward service/resilienceplane 8080:8080
```

A database address of `127.0.0.1` in the Secret points at the API pod itself, not the host. The Compose database address cannot be copied unchanged into a Kubernetes pod. Database installation and TLS are an explicit prerequisite for this chart. The chart does not represent a fully automated full-local profile.

The included ingress NetworkPolicy allows port 8080 only from same-namespace pods labeled `resilienceplane.io/api-client=true`. It requires a CNI implementing NetworkPolicy; default Kind networking does not enforce it. Egress restrictions and API-server address rules remain to be added and validated.

## Teardown

Stop the host API with Ctrl+C, run `make down`, then `make kind-down`. These commands preserve PostgreSQL data by default and delete only the named Kind cluster. To intentionally erase local database state after backing up anything required, run `docker compose down --volumes` from this repository. This deletes the project-managed PostgreSQL volume.
