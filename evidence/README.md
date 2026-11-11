# Evidence index

Primary final-state evidence:

- final-integration.txt — final Go race suite with actual PostgreSQL and actual Kubernetes API/etcd, including explicitly supplied Deployment status.
- http-smoke.json — actual Go process, PostgreSQL, CLI and Next.js proxy;30 sequential snapshot timings are a tiny sample, not throughput capacity.
- go-build.txt, go-vet.txt, go-mod-verify.txt — executable builds, static vet and downloaded-module integrity.
- security-final-binary.txt, security-final-cli.txt — final binary-mode govulncheck results; interpret their scope, not a security certification.
- postgresql-backup-restore.txt — isolated pg_dump/restore state and audit round-trip.
- performance.txt and environment.txt — fixed two-CPU in-process Memory snapshot benchmark and environment limits.
- helm-lint.txt, helm-rendered.yaml, delivery-checks.txt, prometheus-*.txt, source-validation.txt — packaging/config/alert checks.
- ../web/VERIFICATION.md and its linked outputs — console tests/build/audit, plus blocked browser inspection.

Historical/subsystem evidence includes postgresql-store.txt, postgresql-workflow.txt, reviewed-integration.txt and kubernetes-*-envtest.txt. Initial security logs labeled security-initial-*.txt and security-intermediate-*.txt refer to earlier dependency selections; final authoritative files are security-final-binary.txt and security-final-cli.txt. The initial full source/callgraph scan terminated with `signal: killed`; no successful source scan is claimed. Ordinary go-tests.txt can legitimately show optional integration skips when their environment variables are absent; final-integration.txt is the no-skip actual integration run.

No raw database archive, kubeconfig, generated .env, bearer token, private key or running infrastructure state backup is included in this source distribution. Test fixture observations are explicitly identified; none is represented as a running workload's production health.
