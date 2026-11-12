# Threat model and security limits

Assets: mutation authority, cluster identity, policy, approval, recovery images, audit, PostgreSQL credentials, bearer tokens. Boundaries: browser/Next proxy; HTTP/core; PostgreSQL; Kubernetes API; external image registry; CI Docker socket.

| Threat | Implemented mitigation | Remaining boundary |
|---|---|---|
| Unauthorized operational request | Constant-time token comparison; server-assigned viewer/operator roles; known-principal denial audited | Local shared role identities, no OIDC, individual users or tenant isolation |
| Forged health or successful outcome | No public observation/execution/verification HTTP endpoints | Controller and Kubernetes API are trusted |
| Replayed action after uncertain write | Persisted execution key, matching annotation, UID/spec fences and atomic JSON Patch | Not distributed exactly-once; malicious actors with Deployment write access can alter markers |
| Changed policy/evidence | Re-evaluate at approval and admission; freshness, opt-in, image, generation and blast-radius checks | In-flight requests cannot be revoked atomically with SQL policy updates |
| Kubernetes privilege escalation | Namespace Role for deployments/CRDs/leader leases; API cannot execute arbitrary commands | Host dev mode inherits current kubeconfig rights; use a disposable context |
| Argo CD/Flux ownership fight | Reject recognized owner labels and tracking annotation | Metadata-based ownership detection is incomplete; operator must assure exclusive ownership |
| Hostile recovery image | Digest pin prevents mutable-tag substitution | No signature, provenance or image trust verification yet; trusted operator declares image |
| Audit tampering | Same-transaction mirror; repository prefix validation; SQL mutation triggers | Migration/runtime DB principal separation incomplete; DB owner can bypass protections; not tamper-proof storage |
| Secrets in logs and Git | Random private ignored env file; generic API/store errors; no token logging | Review CI logs and deployment secrets; generated SBOM/license and image scans still required |
| Resource exhaustion | Admission bucket, request size/timeouts, bounded connection pool and inventory | Global shared quota can starve peers; no multi-tenant fairness; audit retention unbounded |
| Network interception | Loopback bindings and explicit development trust boundary; chart ingress policy | Compose API/PG plaintext; production TLS/mTLS and restrictive egress not implemented |

All remediation is restricted to test environments and default dry-run. There are no shell/runbook, cloud or destructive chaos endpoints. Unknown bearer credentials return401; known denied principals generate a durable operational denial record. Authentication failures are counted but not stored as identity audit events. Prometheus and health routes are unauthenticated for the local profile and disclose operational counters; keep them private.

This is an early lab project. Do not expose it to untrusted networks or grant production credentials. Apache-2.0 source licensing does not override external component or dependency license obligations. See NOTICE and web/VERIFICATION.md for attribution review limits.

Configuration is validated before database initialization: a dedicated DATABASE_URL is required explicitly; invalid boolean/namespace settings and weak or identical development tokens fail startup before any migration. This avoids relying on libpq/pgx default database discovery or silently disabling Kubernetes integration on a misspelled option.


The TRS CI workflow uses disposable hosted runners with read-only repository permissions, immutable action pins and no persisted checkout credentials. Fork pull requests receive no project secrets. The local lab generates private runtime credentials and kubeconfig under ignored `.lab/`; that directory is also explicitly excluded from Docker build contexts and CI evidence uploads. A Docker-capable host is still required to execute and validate these runtime boundaries.
