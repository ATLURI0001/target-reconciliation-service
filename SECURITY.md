# Security policy

TargetReconciliationService is an experimental local-lab control plane. No release currently carries a production support commitment. Development bearer tokens are not OIDC, do not provide per-user identity, and must not be exposed on a public network.

Report suspected vulnerabilities privately to root@ramprasadatluri.dev with affected version, reproduction, impact, and redacted evidence. Do not include usable credentials or private infrastructure data. Response and fix timelines are not guaranteed.

Local API binding is loopback by default. Keep PostgreSQL private. Production identity, TLS termination, credential rotation, external audit retention, tenant isolation, and independent security review remain release requirements. Kubernetes access is namespace-scoped and meaningful actions default to dry-run with approval required. Never run lab chaos against production resources.
