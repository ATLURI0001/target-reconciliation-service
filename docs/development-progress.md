# Development progress and continuation

## Current checkpoint — 2026-11-12, TRS 0.2.0-dev

The project is now TargetReconciliationService (TRS), repository/module suffix `target-reconciliation-service`, CLI `trsctl`, chart/image `target-reconciliation-service`, group `reliability.trs.io`, and annotation/opt-in prefix `trs.io`. Configuration uses `TRS_`; the sample namespace is `trs-demo`. See rename-migration.md before reusing old installations. Durable SQL table identifiers and historical evidence are deliberately retained.

Completed this execution:

- Added GitHub Actions for push and pull request, with build/test/lint/race/Helm, real PostgreSQL/envtest/image validation and a separate Docker/Kind failed-rollout job. Actions and CI tools are pinned; source passes actionlint and the core commands pass locally. No hosted run was triggered.
- Added genuine CRD status subresource/schema, Ready/Reconciled conditions, observedGeneration, scoped status RBAC, conflict retries, stable transition times, no-op status writes and generation predicates. Real Kubernetes API and PostgreSQL integration passes with the race detector. Envtest still supplies Deployment status; it does not run Pods.
- Added isolated lab startup/demo/teardown with temporary private credentials, private kubeconfig, exact named resources and sanitized evidence. The demo requires actual service HTTP, failure, viewer denial with unchanged image, authorized recovery, matching digest, current-generation Ready and audit.
- Corrected a sample pod-selector mismatch, plus backend/frontend health that could mistake an old available Pod for a healthy new rollout. Strict total/updated replica convergence is now required.
- Renamed the API, CLI, console, charts, configuration, metrics, tests and documentation consistently. Console clean install,7 tests, typecheck, build and HTTP routes pass. Real Go/PG/CLI/Next proxy smoke passes.

Docker/Kind is still blocked here. A genuine user-local Docker/rootless launch failed because UID mapping prerequisites are missing; a cgroup2 mount attempt was denied. Lab startup fails before creating project resources. No successful demo.log or live-Pod recovery is claimed. Details are in evidence/trs-docker-attempt.txt and trs-lab-attempt.txt. No privilege escalation or policy bypass was requested, no cloud resources were created, and nothing was published.

## Resume order

1. Read implementation-status.json, verification.md, CI.md, controllers/README.md and local-lab.md. Current evidence is `evidence/trs-*`; prior milestone evidence is unchanged under evidence/history/.
2. On a Docker-capable machine, install prerequisites and run `make lab-up`, `make lab-demo`, `make lab-down`. Record actual success and inspect any failures. Do not treat the supplied CI source or envtest as a successful running-workload demonstration.
3. After separately authorized GitHub publication, inspect both Actions jobs and retain their artifact for the final commit. Source actionlint/local commands are already validated; hosted execution is not.
4. Finish production identity/runtime DB privileges before wider exposure, then outbound agents and two-cluster fencing/routing. NATS/outbox, Argo CD delivery, Terraform/Ansible/cloud paths, ELK/OTel/SLOs remain separate milestones.

## Earlier checkpoint

The 2026-11-11 ResiliencePlane 0.1 milestone implemented the underlying durable observe/plan/approve/execute/verify/audit path. Its exact report, scan history, backup/restore and benchmark results are preserved under evidence/history/resilienceplane-0.1.0. They are historical evidence, not renamed proof of this release. Git history is preserved in the repository bundle.
