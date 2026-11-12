# TargetReconciliationService (TRS) rename — 0.2.0-dev

This revision adopts the user-selected name and adds CRD conditions and automated CI/lab entry points. It does not claim exclusive ownership or worldwide availability of the name or the `trs.io` domain.

| Element | Previous | Current |
|---|---|---|
| Display name | ResiliencePlane | TargetReconciliationService (TRS) |
| Repository/module suffix | resilienceplane | target-reconciliation-service |
| Go module | github.com/ATLURI0001/resilienceplane | github.com/ATLURI0001/target-reconciliation-service |
| Management executable | resilienceplane | target-reconciliation-service |
| CLI | rpctl | trsctl |
| Helm chart/release example | resilienceplane | target-reconciliation-service |
| CRD API group | reliability.resilienceplane.io | reliability.trs.io |
| Execution annotation | resilienceplane.io/execution-key | trs.io/execution-key |
| Remediation opt-in label | resilienceplane.io/allow-remediation | trs.io/allow-remediation |
| Environment prefix | RP_ | TRS_ |
| Metric prefix | resilienceplane_ | trs_ |
| Demo namespace | rp-demo | trs-demo |
| Local Kind cluster | resilienceplane-lab | target-reconciliation-service-lab |
| Docker image/Compose project | resilienceplane | target-reconciliation-service |

`DATABASE_URL`, `TEST_DATABASE_URL`, `KUBEBUILDER_ASSETS` and standard Kubernetes variables retain their names. PostgreSQL's internal `rp_state` and `rp_audit` table identifiers are intentionally retained to preserve the persistence contract; branding alone is not a reason to rewrite durable audit data. Historical test logs and Git commits retain the old name and module paths as recorded.

## Existing installation boundary

This is a breaking development release, not a transparent in-place migration of active recovery operations. Stop the old controller and disable/cancel old automation before changing installation identity. Do not run old and new controllers concurrently against the same opted-in workload. Preserve backups and inspect actual Deployment annotations before resuming any recovery.

For local development, use a fresh TRS Kind cluster/namespace and its separate Compose project. The scripts affect only the new named project; old clusters and volumes are not deleted automatically. Generate a new ignored `.env` with `make init`; do not copy the old `RP_` file unchanged or include it in source. If retaining any credentials intentionally, migrate the variable names privately and review all values; no automated secret migration is performed.

The new CRD group is a new Kubernetes API identity. Install the new CRD and create new ReliabilityTarget resources using `reliability.trs.io/v1alpha1` after inspecting their deployment/container/image references. Kubernetes does not rename a CRD's group in place. New conditions are available under `status` and the standard status subresource. Do not copy old status into new resources.

If reusing a database deliberately, `rp_state`/`rp_audit` remain readable, but recorded cluster/workload/namespace identities are not rewritten. The new controller only manages `trs-demo`; old records remain historical, and old active incidents must not be silently replayed as TRS actions. A dedicated new database is recommended for the renamed disposable lab. Restore/upgrade of an existing non-lab installation requires a reviewed migration plan outside this release's scope.

Prometheus/Grafana configurations must switch to `trs_*` queries; retained time series keep their historical names. The archive includes preserved earlier evidence under `evidence/history/` and current TRS evidence separately.
