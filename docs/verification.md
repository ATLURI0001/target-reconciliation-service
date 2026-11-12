# TargetReconciliationService (TRS) — verification and handoff

2026-11-12 · 0.2.0-dev. This is an updated, tested development milestone; the full multi-cloud flagship roadmap remains incomplete.

## A. Implemented functionality

| Requested change | Actual result |
|---|---|
| Rename the existing project | Implemented across source/module, API executable, `trsctl`, chart, image, group/annotations, configuration/metrics, console, tests and docs. Historical evidence and durable SQL identifiers are intentionally preserved. |
| GitHub Actions push/PR CI | Workflow implemented with pinned read-only actions, build/test/lint/race/Helm, real PostgreSQL/envtest/image validation and a separate Docker/Kind live recovery job. actionlint and core commands passed locally; hosted execution has not occurred. |
| CRD status conditions | Implemented and verified against actual Kubernetes API: status subresource, Ready/Reconciled, observedGeneration, stable transitions, optimistic retries, no-op writes, event predicate and scoped status RBAC. |
| Actually execute Docker/Kind and record successful demo.sh | **Blocked**. Genuine rootless launch failed during UID/GID mapping; cgroup2 mounting denied. Actual lab entry point failed before creating infrastructure. No successful demo run is included. |

The prior API/CLI, PostgreSQL workflow, policy/approval/execution/verification and audit remain operational in their tested scope. A readiness defect was corrected: an old available Pod plus an unready new Pod cannot be reported as a completed rollout. Total and updated replicas must both equal desired. The sample selector mismatch was also fixed. The console mirrors the stricter rule.

## B. Repository artifact

`target-reconciliation-service-0.2.0-dev.zip` contains current source, Go/npm locks, charts, CI/lab scripts, tests, reports, implementation-status.json, raw current evidence and unchanged prior evidence. A repository.bundle preserves local Git history. Source dependencies/build caches, application binaries, credentials, kubeconfigs and databases/dumps are excluded. There is no remote repository or published image.

| Element | Current value |
|---|---|
| Full name / abbreviation | TargetReconciliationService / TRS |
| Repository | target-reconciliation-service |
| Go module | github.com/ATLURI0001/target-reconciliation-service |
| API executable | target-reconciliation-service |
| CLI | trsctl |
| Helm chart | target-reconciliation-service (chart 0.2.0) |
| CRD group | reliability.trs.io |
| Annotation / opt-in | trs.io/execution-key / trs.io/allow-remediation |
| Docker image | target-reconciliation-service |
| Environment / metrics | TRS_ / trs_ |
| Demo namespace | trs-demo |

The rename is a breaking development boundary. It is not an automatic migration of active recovery plans. Read rename-migration.md before reusing an old installation or database. No name exclusivity or domain ownership is asserted.

## C. Architecture summary

The management API and namespace-scoped Kubernetes reconciler share the reliability service and PostgreSQL transactions. Approval/admission/audit are durable and serialized. Kubernetes changes occur after commit with UID, generation, image and current resourceVersion fences plus a durable execution marker. The console reads the actual API. New CRD status reports reconciliation/health evidence; it does not authorize operations or certify application-level health. Full API outages cannot be written back as a new Kubernetes condition; backend freshness remains relevant.

## D. Validation actually executed

| Check | Result | Evidence |
|---|---|---|
| `make build test lint helm` | Passed with actual PostgreSQL and envtest assets configured | evidence/trs-ci-local.txt |
| `go test -p 1 -race -count=1 -v ./...` | Passed, 37 top-level tests plus subtests; no skips | evidence/trs-final-integration.txt |
| Kubernetes status/subresource/RBAC/atomic patch | Passed against Kubernetes API 1.35.0 and etcd; real PostgreSQL variant also passed | evidence/trs-final-integration.txt |
| Actual Go HTTP, PG, trsctl and Next server proxy | Passed | evidence/trs-http-smoke.json |
| Console clean install,7 model tests, typecheck, production build and HTTP routes | Passed | web/VERIFICATION.md and evidence/trs-web-* |
| `go mod verify` | All modules verified | evidence/trs-go-mod-verify.txt |
| Workflow actionlint 1.7.12 and source assertions | Passed | evidence/trs-actions-lint.txt; trs-ci-source-validation.json |
| Helm lint/render, canonical/chart CRD match | Passed; chart metadata updated to 0.2.0 | evidence/trs-helm.txt; trs-helm-final.txt |
| CI Helm/Kind/kubectl downloads | Installer ran and verified pinned SHA256 values | scripts/install-ci-tools.sh and docs/CI.md |
| Shell syntax and JSON/YAML parsing | Passed; templates checked through Helm | evidence/trs-source-validation.txt |
| govulncheck 1.1.4 binary mode | No known vulnerabilities reported for current API binary | evidence/trs-security-binary.txt |
| npm production dependency audit |0 reported at execution time | evidence/trs-web-audit.json |
| Local Docker/rootless and make lab-up | Failed/blocked before containers or cluster ran | evidence/trs-docker-attempt.txt; trs-lab-attempt.txt |
| Hosted GitHub Actions | Not triggered; no source publication authorized | docs/CI.md |

Current environment: shared Linux amd64 workspace; Go 1.27.4, PostgreSQL 17.11, Kubernetes API 1.35.0, Node 24.22.1. All local database/API test processes were stopped after checks. Envtest has no scheduler or kubelets and explicitly injects Deployment rollout status. These tests are real API/SQL verification, not live running-Pod recovery. The HTTP proxy smoke is a server/transport check, not browser visual inspection.

The small actual snapshot sample comprised 30 sequential loopback requests against synthetic integration-test records (58 cluster records and 3 observations), median 0.927ms and nearest-rank p95 1.294ms. This is not throughput capacity or a recovery-time guarantee. Earlier backup/restore and Memory benchmark evidence remains historical under evidence/history/; those procedures were not rebranded or rerun as new results.

## E. Known limitations and blockers

The Docker/Kind requirement remains unfulfilled in this environment. Installing user-local Docker binaries did not provide a working daemon. Rootless startup failed at missing newuidmap, and a cgroup2 mount attempt was denied. The supplied lab scripts and hosted job must still execute on a capable Docker host; a configured workflow is not a green hosted run. No successful demo.log, pod recovery, application RTO or two-cluster claim is made.

The current CI is a validation pipeline, not a complete GitHub/Jenkins/Argo CD delivery system. OIDC/mTLS, tenant isolation, agents, NATS/outbox, GitOps promotion, full SLO/ELK/OTel integrations, cloud Terraform/Ansible, stateful failover, capacity billing and full management pages remain separate planned/partial milestones. Storage still uses a serialized JSONB aggregate. Container scans/SBOM/signing, broad secret/license review, browser visual inspection and a fresh full source-callgraph security scan are not completed by the current binary/npm checks. Production readiness is not claimed.

## F. GitHub readiness

Apache-2.0 original source, NOTICE, community templates, architecture/ADRs, security/threat model, migration/runbooks and release instructions are present. The new workflow triggers on push and pull request and requires only read permissions; it never publishes an image or provisions cloud infrastructure. Current source is ready for review, with third-party attribution and runtime release gates outstanding. No remote repository, push, tag, hosted workflow run, billable infrastructure or production mutation occurred.

## G. Next engineering milestone

Execute the exact isolated lab on a Docker-capable host, retain the real demo/HTTP/Ready-generation/approval/audit artifacts, and fix any observed runtime failures. Then, after separately authorized publication, verify both Actions jobs on the final pushed commit. Only that evidence can close the two remaining runtime/hosted-execution gaps. Continue other platform milestones from implementation-status.json and development-progress.md.

## H. Reproduction

Prerequisites: Go 1.27.4+, Git, Python3, Docker Engine/Desktop with Compose v2, kubectl, Kind and Helm 3; Node 22.13+/npm for the console. Use Linux/macOS or Windows WSL2. The pinned CI tool installer is Linux/amd64 only; other hosts install native tools.

```sh
make build test lint helm
make lab-up
make lab-demo
make lab-down
```

`lab-demo` explicitly requests a failure and approval inside only the named disposable cluster. It captures actual service HTTP before/after, current-generation CRD conditions, policy denial with unchanged image, approved digest rollback, verification and audit. Teardown deletes that lab's temporary database; existing unrelated resources are refused/preserved. Read local-lab.md for safe boundaries and abort behavior.

For the console:

```sh
cd web
npm ci
npm test
npm run typecheck
npm run build
npm start
```

For real local API/SQL tests without kubelets, use a disposable TEST_DATABASE_URL and KUBEBUILDER_ASSETS, or `make integration envtest`. Ordinary unit runs explicitly skip unavailable external integrations; only the recorded combined run above had no skips. Full CI sources and command reproduction are in CI.md. No remote publication is required to inspect/build this source.
