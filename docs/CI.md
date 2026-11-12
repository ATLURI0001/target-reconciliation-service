# Continuous integration

`.github/workflows/ci.yml` runs on every push and pull request when this repository is hosted on GitHub and Actions is enabled. It has no path or branch filters. Supplying this workflow does not demonstrate a hosted run: no repository was created, code published, or Actions run triggered during this development session.

## Jobs

| Job | Executable checks |
|---|---|
| Go, PostgreSQL, envtest, Helm, and image build | `make build`, `make test`, `make lint`, `make race`, `make helm`; generated local credentials and a real PostgreSQL Compose database; `make integration`; Kubernetes API server/etcd via `make envtest`; `docker build` |
| Real failed rollout, denial, approval, and verified recovery | `make lab-up`, `make lab-demo`, always `make lab-down`; upload only `evidence/ci-lab/` for seven days |

Jobs use disposable GitHub-hosted Ubuntu 24.04 runners. The Kind job genuinely requires Docker, a kubelet, a container runtime, registry access, and Linux cgroup v2. Envtest starts only an API server and etcd, so it does not substitute for this job's Deployment rollout or service HTTP verification. A failed or blocked lab check fails its job; no mock replaces it. PostgreSQL integration credentials are random for each run and erased during cleanup. The pipeline does not publish images, update GitOps, provision cloud infrastructure, or deploy to any external cluster.

`make lint` checks formatting without modifying source and then runs `go vet`. Shell error propagation includes `pipefail`; a failed formatter or vet process cannot be hidden by an empty output check.

## Trust and dependency pins

The workflow requests only `contents: read`, uses `pull_request` rather than `pull_request_target`, and disables checkout credential persistence. Fork pull requests receive no project secrets. Docker access is restricted to disposable hosted runners; do not casually move these jobs to a privileged, shared self-hosted runner.

Action commit SHAs were resolved against the official GitHub REST tag-ref endpoints on 2026-11-12:

| Action | Reviewed reference | Pinned commit |
|---|---|---|
| actions/checkout | v4 | `11d5960a326750d5838078e36cf38b85af677262` |
| actions/setup-go | v5 | `40f1582b2485089dde7abd97c1529aa768e1baff` |
| actions/upload-artifact | v4 | `ea165f8d65b6e75b540449e92b4886f43607fa02` |

Sources: `https://api.github.com/repos/actions/checkout/git/ref/tags/v4`, `https://api.github.com/repos/actions/setup-go/git/ref/tags/v5`, and `https://api.github.com/repos/actions/upload-artifact/git/ref/tags/v4`. Commit pinning prevents a future tag movement from silently changing executable workflow code; it does not replace review of upstream code.

`scripts/install-ci-tools.sh` installs into project-local `.tools/bin/` without sudo. It supports Linux/amd64 and verifies hardcoded SHA256 values before executing downloaded binaries:

| Tool | Version | SHA256 |
|---|---|---|
| Helm archive | 3.17.3 | `ee88b3c851ae6466a3de507f7be73fe94d54cbf2987cbaa3d1a3832ea331f2cd` |
| Kind binary | 0.31.0 | `eb244cbafcc157dff60cf68693c14c9a75c4e6e6fedaf9cd71c58117cb93e3fa` |
| kubectl binary | 1.35.0 | `a2e984a18a0c063279d692533031c1eff93a262afcc0afdc517375432d060989` |

Checksums were retrieved from the official Helm download endpoint, official Kind release assets, and `https://dl.k8s.io/release/v1.35.0/bin/linux/amd64/kubectl.sha256`. The Kind node image is pinned to the Kubernetes 1.35.0 digest published in the Kind 0.31.0 release notes. These are compatibility selections, not assertions that they are the latest releases.

Go is read from `go.mod`. Its dependency graph is locked by `go.sum`. Runtime base images and the PostgreSQL image currently use explicit version tags; their immutable digest pinning and supply-chain scans remain release requirements. The installer is not a general cross-platform dependency manager; macOS and Windows/WSL2 developers should install the documented native tools.

## Reproducing checks

```sh
make build test lint race helm
make init
docker compose up -d --wait postgres
set -a; . ./.env; set +a
export TEST_DATABASE_URL="postgres://target-reconciliation-service:${TRS_DATABASE_PASSWORD}@127.0.0.1:5432/target-reconciliation-service?sslmode=disable"
make integration envtest
docker build -t target-reconciliation-service:local .
docker compose down --volumes
make lab-up
make lab-demo
make lab-down
```

The explicit `--volumes` commands erase their project-managed test databases. Use isolated local test state and back up anything needed first. The lab uses a separate Compose project (`trs-lab`), temporary private credentials and kubeconfig under ignored `.lab/`, and a named local Kind cluster. See [local-lab.md](local-lab.md) for operational boundaries and evidence.

For CI status after an authorized push, inspect the Actions page and the two job results. Repository branch protection is an administrator decision; this source change does not enable required checks or prove that an observed green status belongs to the final commit.
