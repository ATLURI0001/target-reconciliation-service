# Recovery runbook

This is a lab runbook; RPO/RTO values below are requirements, not guarantees. Workflow service reconstruction, database reconnection and execution-key replay are tested. There is no full infrastructure disaster-recovery guarantee.

| Failure | Current behavior | Target/limitation |
|---|---|---|
| Process restart | Workflow and audit in PostgreSQL; leader election; same execution key replays without another image patch | Target control-plane restart within 120s; no process-failure RTO measurement yet |
| Kubernetes API interruption | Uncached observation fails; existing evidence ages, no fresh health invented, approved actions require freshness | Target stale indication within freshness 30s plus next observation/UI refresh; no live outage measurement |
| Uncertain patch response | Deployment marker and image inspected on next attempt; same-key replay | API/SQL transaction gap exists; not exactly-once delivery |
| Failed rollout | Test-only operator-approved digest rollback and readiness verification | Two-minute execution/verification deadline; no request-level recovery guarantee |
| PostgreSQL unavailable | API writes/readiness fail; no memory fallback | Recover database before controller processing resumes |
| Whole cluster/stateless serving loss | No automated route/failover operation | Requires agent, routing and fencing milestone |
| Stateful datastore loss | No automatic promotion or failover | Requires backup integrity, ownership, replication and split-brain validation |

Before any intervention: inspect freshness and target UID, incident evidence and audit; verify selected kubecontext and ownership. Apply emergencyStop through a healthy authenticated API or stop controllers if the API is unavailable. Stopping a process cannot retract already-sent Kubernetes operations. Observe the actual target before resuming; do not blindly rerun an old plan against a replaced Deployment.

Back up PostgreSQL with a consistent `pg_dump` snapshot and encrypted offline retention. Restore into a separate empty database, compare rp_state and rp_audit, run schema/version and health checks, and only then configure a stopped management plane to use that database. Restoring an old database can resurrect approvals: default to disabled execution/emergency stop and inspect real Kubernetes execution annotations before resuming. Database backup RPO is the backup interval; RTO depends on measured restore and integrity verification. No values have been promised. Never commit dumps, kubeconfigs or credentials.

The isolated backup/restore test, when present in evidence, verifies one database snapshot round-trip only. It is not a scheduled backup system, off-site retention, point-in-time recovery or tested disaster failover.
