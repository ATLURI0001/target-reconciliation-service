# Optional local metrics profile

```sh
make metrics-up
```

Open Prometheus at http://127.0.0.1:9390 and Grafana at http://127.0.0.1:3001. Grafana user is `admin`; read the generated `RP_GRAFANA_PASSWORD` value privately from `.env`. `scripts/init-local.sh` safely adds this key to an older `.env` without rotating existing database or API credentials. Grafana stores its initial password in its persistent volume: changing `.env` later does not reset an existing Grafana administrator password.

The provisioned `ResiliencePlane · Core Reliability` dashboard queries actual API counters and PostgreSQL-backed cluster, Deployment, and incident observations. No metrics are seeded. No cluster observations in Compose's API-only mode means empty cluster panels. Missing observations are not rendered as zero or successful health. To inspect real Kubernetes observations, scrape the Kubernetes-enabled control-plane endpoint from a reachable Prometheus environment; the provided Compose scrape target is the Compose API service, not a host process or Helm service.

Metrics are currently accessible without authentication within the local Docker network and loopback-published API. Do not expose these services publicly. Prometheus is not configured with API authentication, TLS, or tenant boundaries. Grafana anonymous access and user signup are disabled. Containers have CPU/memory limits; Prometheus retention is bounded to 24 hours and 256 MB of stored time series, which does not bound all WAL or head-block overhead. Volume disk use still needs host monitoring.

## Queries and methodology

- `rate(resilienceplane_http_requests_total[5m])`: management API request rate; not business service throughput.
- `rate(resilienceplane_http_failures_total[5m]) / clamp_min(rate(resilienceplane_http_requests_total[5m]), 0.000001)`: process-local API failure counter ratio. This is not an application availability SLO.
- `time() - resilienceplane_cluster_last_seen_seconds`: age of last persisted observation per registered cluster. It is a freshness indicator, not a direct connectivity probe.
- `resilienceplane_deployment_available_replicas` compared with `resilienceplane_deployment_desired_replicas`: last observed Deployment state. Check observation age before acting.
- `resilienceplane_incidents{state="Resolved"}`: persisted incident count by state, not an incident-resolution rate.

The telemetry alert uses the default policy's 30-second freshness limit plus a 20-second alert hold. If the runtime policy changes, update the alert threshold separately. Alert rules do not execute remediation and no Alertmanager receiver is configured. The controller uses its own deterministic, policy-governed observations rather than treating a Prometheus alert as authorization.

## Verification

With Prometheus tooling installed, run `promtool check rules observability/prometheus/rules.yaml` and `promtool test rules observability/prometheus/rules.test.yaml`. The rule tests verify stale telemetry and missing scrape targets have distinct alerts and respect their hold periods. Validate the mounted configuration using `docker compose --profile metrics run --rm --no-deps --entrypoint promtool prometheus check config /etc/prometheus/prometheus.yaml`. After startup, check Prometheus targets for `up{job="resilienceplane"} == 1`, query the counters, and inspect the provisioned Grafana dashboard. Record real results before claiming the integration works end to end.

`make metrics-down` stops project services and preserves data. `docker compose --profile metrics down --volumes` intentionally destroys all project-managed database, Grafana, and Prometheus volumes and should only be used when that loss is desired.
