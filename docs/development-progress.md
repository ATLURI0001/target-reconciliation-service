# Development progress and continuation

2026-11-11 —0.1.0-dev. Resume from the source and evidence, not from assumed roadmap completion. Read README, ARCHITECTURE, implementation-status.json, docs/verification.md and threat-model.md first.

## Completed initial product path

Trusted uncached Deployment observation → sustained fresh failure → deterministic digest rollback plan → policy evaluation → identity-linked approval → serialized admission → atomic Kubernetes patch with execution marker → readiness verification → PostgreSQL audit timeline. API, CLI and read-only console expose the persisted state. Real Kubernetes API/etcd/PostgreSQL integration tests exercise the full control-plane path; rollout status is explicitly supplied because envtest has no kubelets.

Safety review corrected recovered-after-approval execution, late-success deadline bypass, container/replica verification binding, unsafe duration ranges, per-target conflicting claims, policy-denial audit order and stale cached health. Regression tests cover these findings. API has bounded requests/concurrency and audit pagination; semantic validation is400. Default action mode remains dry-run.

## Verification evidence

The final report distinguishes actual binaries/services, fake test input and unavailable runtime validation. Evidence logs remain in evidence/ and web/. Source-level vulnerability analysis was terminated by resource limits; binary-mode and npm audits have separate results and are not a complete security certification. Initial outdated-toolchain findings prompted the current Go requirement and updated transitive dependencies; retain historical scan context as labeled evidence.

## Environment facts

The original environment had no Go toolchain (GNU Go was a different executable), Docker daemon, kubectl or Kind. Go archives, Helm, envtest and PostgreSQL were obtained into transient task tooling and checksums/versions checked where documented. They are not packaged as repository dependencies. Developer machines must install prerequisites. All helper database instances and test API processes are disposable loopback processes; production credentials were never requested or used.

## Next execution

1. Read verification/status and reproduce build, unit/race, PG and real API tests with pinned dependencies.
2. On an actual Docker machine, run `make up`, then the named Kind lab. Complete scripts/demo.sh with a real healthy baseline and bad rollout; capture actual pod/request evidence and console timeline, and fix observed failures.
3. Complete install/teardown automation and runtime scans before expansion. Add OIDC identities and separate runtime migration privileges before wider exposure.
4. Only then implement outbound enrolled agents and two-cluster stateless coordination with fencing, before NATS/outbox and reviewed GitOps delivery. Avoid inventing provider modules until the first provider has a complete validation plan.

No remote repository or release may be created without explicit user authorization. Publication readiness still requires dependency/license review, actual delivery scenario and truthful release notes. The user requested all 12 DevOps technologies; the initial slice deliberately marks unfinished integrations planned rather than claiming decorative configs are working integrations.

Final dependency review raised the toolchain minimum to Go 1.27.4, pgx to5.9.2 and x/net to0.62.0 with compatible transitive dependencies. Startup configuration now fails before database access on missing explicit DATABASE_URL, invalid Kubernetes boolean/namespace or invalid development credentials. Tests in cmd/resilienceplane cover this boundary without opening a database.
