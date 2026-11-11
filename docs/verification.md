# ResiliencePlane verification and handoff

2026-11-11 · 0.1.0-dev · First control-plane milestone, not full flagship release.

## A. Implemented functionality

Actual Go API and rpctl; PostgreSQL cluster/observation/policy/incident/audit persistence; controller-runtime ReliabilityTarget reconciler; uncached health reads; deterministic fresh-failure detection; safety policy, dry-run, explicit approval, admission, idempotent digest rollback and attributed readiness verification; identity-linked durable audit; read-only Next.js console. Source includes resource-limited Docker/Compose, Helm, Jenkins validation, optional Prometheus/Grafana configuration, community files and reproducible tests.

Safety invariants tested include denied authorization with no mutation, stale evidence, opt-in and GitOps exclusion, protected namespace, target UID/spec/image/replica changes, concurrent same-target claims, late verification, recovered-after-approval, API read failure, audit immutability and replay after new service/database connection.

## B. Repository artifact

The source archive contains the monorepo, pinned Go/npm dependencies, tests, docs, machine-readable implementation-status.json and raw evidence. Generated .env files, kubeconfigs, databases/dumps, private keys, dependency directories, build caches and binaries are excluded. The executable is rebuilt with `make build`. Local Git is initialized; no remote/publication exists.

## C. Architecture summary

The console/CLI use an authenticated management API. API and namespace-scoped Kubernetes reconciler share the reliability service and PostgreSQL repository. SQL transactions serialize policy/approval/admission and audit. Kubernetes mutations occur after commit with UID/resourceVersion/image CAS and a matching execution marker. Kubernetes evidence is eventually consistent; the SQL aggregate is strongly serialized. There is no implemented NATS service, outbound agent, OIDC, multi-cluster failover or cloud path. See ARCHITECTURE.md and ADRs.

## D. Executed validation

Environment: Linux amd64 shared virtual runtime, Go 1.27.4, PostgreSQL 17.11, Kubernetes API 1.35.0/etcd 3.6.6, Node 24.22.1. Test binaries/servers were obtained in task scratch and are not bundled. Exact versions, commands and raw evidence are retained.

| Command or procedure | Outcome | Evidence |
|---|---|---|
| `make build` with Go 1.27.4 | API and CLI built | evidence/go-build.txt |
| `go test -p 1 -race -count=1 -v ./...` with TEST_DATABASE_URL and KUBEBUILDER_ASSETS | Passed; 30 top-level tests, no integration skips | evidence/final-integration.txt |
| `go vet -p 2 ./...` | Passed, no diagnostics | evidence/go-vet.txt |
| `go mod verify` | All downloaded modules verified | evidence/go-mod-verify.txt |
| Actual HTTP/PG/CLI/Next proxy smoke | Passed; known viewer mutation denied; semantic validation; audit pagination; real metrics | evidence/http-smoke.json |
| Real Kubernetes API checks | Atomic stale-RV rejection, CRD schema, namespace/Secrets RBAC, workflow and uncached-failure regression passed | evidence/final-integration.txt |
| PostgreSQL pg_dump/pg_restore into separate DB | State/revision/ordered audit digests match | evidence/postgresql-backup-restore.txt |
| Console clean install,5 model tests, typecheck and production build | Passed | web/VERIFICATION.md and linked logs |
| Helm lint/render | Passed,9 rendered objects including CRD | evidence/helm-lint.txt and helm-rendered.yaml |
| Prometheus config/rule checks and 2 semantic rule tests | Passed | evidence/prometheus-*.txt |
| Source JSON/YAML parsing and shell syntax | Passed; Helm templates checked separately | evidence/source-validation.txt |
| govulncheck1.1.4 binary mode, API and CLI | No vulnerabilities reported for final binaries/manifest | evidence/security-final-binary.txt and security-final-cli.txt |
| npm production dependency audit |0 reported vulnerabilities at run time | web/audit-production.json |

The actual Kubernetes API tests explicitly set Deployment status as test input: envtest has no scheduler, kubelet or running application. They exercise actual API/SQL mutation and persisted workflow decisions, not actual pod recovery. The Next proxy smoke exercises the running server and real backend, not browser interactions.

Recorded storage baseline (1000 synthetic entries, in-process Memory snapshot only, GOMAXPROCS=2):

| Iterations | ns/op | B/op | allocations/op |
|---|---|---|---|
| 745 | 2965241 | 1029861 | 4086 |
| 776 | 3133418 | 1030492 | 4086 |
| 685 | 3094605 | 1035617 | 4086 |

The actual PostgreSQL-backed loopback snapshot sample had 30 sequential requests, 1 registered cluster, 0 observed targets and 2 audit entries; median 0.354ms, nearest-rank p95 0.645ms. This tiny read sample and Memory copy benchmark establish no production capacity, concurrent-cluster scale or recovery-time guarantee. See docs/performance.md.

## E. Known limitations and blocked checks

Docker daemon/Kind/kubelets unavailable: no fresh image/Compose runtime, running workload recovery, actual logs/traffic recovery or two-cluster failover was demonstrated. Grafana runtime and browser visual inspection were not verified. Jenkins/GitHub/Argo CD delivery is not executed. Full source/callgraph vulnerability scanning was terminated by resource limits; binary-mode scans and npm audit are narrower checks, not security certification. Trivy/image scans/SBOM/signing and comprehensive license/secret review remain release checks; a basic source pattern scan is not a substitute.

OIDC, multi-tenant isolation, outbound mTLS agents, NATS/outbox, OTel, ELK, request SLIs/SLOs, topology/deployment intelligence, cost/billing analysis, Terraform/Ansible/cloud modules, controlled chaos and route failover are incomplete/planned. Every requested deliverable is classified in implementation-status.json. The first database stores a serialized JSONB aggregate and rewrites retained audit; it is not a scale-ready normalized store. Audit retention, timeout sweeper and per-principal fairness are pending. Migration owner can bypass audit triggers. Cancellation cannot retract an in-flight Kubernetes request. Known-good image trust is operator-supplied, not automatically certified.

## F. GitHub readiness

Apache-2.0 original-source license, NOTICE, README, architecture/ADRs, contribution/support/security/conduct docs and issue/PR templates are present. Third-party dependency/image/optional-component attribution review is unfinished. The provisional name was searched without a conclusive comprehensive registry/trademark check. Local source is reviewable; no remote repository, push, tag, image publication or billable infrastructure was created. Proposed publication gates are in docs/release.md. This is not a claim of a finished public flagship release.

## G. Continuation plan

Next complete a fresh one-cluster Docker/Kind installation and real failed-rollout demonstration, including denied viewer mutation, explicit approval, actual pod/request recovery, persistent audit and console timeline. Record installation/teardown and observed recovery times. Then implement production identity and outbound agent enrollment, followed by two-cluster stateless fencing/routing. Expand durable events and reviewed GitOps delivery only after the prior slice is reproducible. See docs/development-progress.md and ROADMAP.md.

## H. Reproduction

Install Go 1.27.4+, Git, Python3, Docker/Compose v2 and Node 22.13+/npm. Use Linux/macOS or Windows WSL2. Install Kind/kubectl for the live lab and Helm 3 for chart checks.

```sh
make doctor
make init
make build
make test
make up
make smoke
cd web
npm ci
npm test
npm run typecheck
npm run build
npm start
```

Console http://127.0.0.1:3000 reads API 8080. Enter the private RP_VIEWER_TOKEN from your generated .env; never commit it. Compose's API-only profile has no fabricated cluster health. For actual Kubernetes integration follow docs/local-lab.md, then explicitly run `RP_DEMO_EXECUTE=yes ./scripts/demo.sh` in the named disposable context. For actual API-server tests, `make envtest`; add a dedicated TEST_DATABASE_URL for durable integration. Ordinary `make test` explicitly skips unavailable integrations and must not be mistaken for the no-skip recorded run. `make down` preserves database volumes; `make kind-down` removes only the named lab cluster.
