# Evidence index — TRS 0.2.0-dev

Current execution,2026-11-12:

- trs-ci-local.txt: actual `make build test lint helm` with real PostgreSQL and Kubernetes API assets configured.
- trs-final-integration.txt: final race suite against actual PostgreSQL 17.11 and Kubernetes API 1.35/etcd; no integration skips. Deployment status is explicitly test input, not running-Pod evidence.
- trs-status-envtest.txt: focused conditions, subresource/generation, RBAC, conflict/no-op/predicate and workflow tests. Its optional PostgreSQL skip is superseded by the combined final run above.
- trs-http-smoke.json: actual Go process, PostgreSQL, CLI and Next server proxy checks. The tiny sequential latency sample uses synthetic integration-test records and is not a throughput/RTO claim.
- trs-go-mod-verify.txt: module cache integrity.
- trs-web-*: current console install,7 model tests, typecheck, production build, route smoke and production dependency audit.
- trs-actions-lint.txt and trs-ci-source-validation.json: workflow source validation, not a hosted Actions run.
- trs-ci-installer.txt/trs-helm.txt/trs-lint-guards.txt (when present): pinned tool/Helm and lint error-propagation checks.
- trs-docker-attempt.txt: actual failed rootless Docker startup, exact missing UID mapping and denied cgroup mount evidence.
- trs-lab-attempt.txt: actual lab entry-point failure before creating infrastructure. No successful running workload recovery was recorded.
- trs-security-binary.txt: current binary-mode vulnerability scan; inspect its actual exit/result, not a security certification.

Historical evidence under history/ is retained verbatim. It includes the earlier project's successful database backup/restore, storage benchmark and security scans, along with explicitly recorded blockers. No .env file, API token, kubeconfig, database dump or raw infrastructure state backup belongs in the source artifact. Git history may legitimately contain the old name; active TRS source uses the new identifiers except intentionally stable SQL schema names.
