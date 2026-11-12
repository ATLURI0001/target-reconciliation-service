# Integration roles and current boundaries

The executable initial slice deliberately prioritizes the policy-governed recovery path. A configuration file or planned role is not evidence of a verified integration.

| Technology | Role | Initial delivery status |
|---|---|---|
| Git | Local source history and reviewable change traceability | Repository initialized; no remote push |
| GitHub | Public source, PRs, issue review, protected releases | Push/PR Actions workflow and templates prepared; actionlint and local core commands passed; hosted execution/publication pending |
| Jenkins | Formatting, vet, race tests, Helm validation, secret/dependency scans, image build, SBOM | Pipeline supplied; requires configured agent and actual execution |
| Docker | Reproducible Go build and local API/PostgreSQL isolation | Dockerfile, Compose and repeatable Kind lab supplied; genuine local Docker attempt blocked; CI includes live Docker/Kind job |
| Kubernetes | Namespace-scoped observation and opt-in remediation | Real API/PG integration, native Ready/Reconciled status conditions and scoped status RBAC verified; live Pods blocked |
| Helm | Versioned control-plane installation | Chart supplied; requires existing database and credential Secret |
| Terraform | Provider infrastructure provisioning | Planned; no cloud modules represented as complete |
| Ansible | Repeatable lab and Jenkins host preparation | Planned; no host changes performed |
| Argo CD | Ownership-aware GitOps delivery | Planned; controller refuses to mutate detected GitOps-owned workloads |
| Prometheus | Real application metrics and alert evidence | Real API metrics plus optional Compose scrape/rule configuration; runtime integration not yet verified |
| Grafana | Backend metrics dashboards | Provisioned optional Compose dashboard against actual metric names; runtime UI not yet verified |
| ELK | Structured operational and audit investigation | Planned; no ingestion pipeline claimed operational |
| NATS JetStream | Durable event fan-out from transactional outbox | Planned; not needed for initial serialized workflow correctness |
| OpenTelemetry | Trace context and workflow instrumentation | Planned |
| OIDC | Production identity architecture | Planned; local bearer-token authentication only |

The remaining delivery scenario is GitHub → Jenkins → scanned immutable image → reviewed GitOps commit → Argo CD → canary verification. The supplied Jenkins pipeline stops before artifact publication or GitOps mutation. Do not describe scenario A as demonstrated until that complete chain is run and evidence recorded.

Image tags in the development Dockerfile and Compose are version-specific but not immutable digests. A release must pin image digests, generate an SBOM, scan the resolved images, and sign or verify the published digest. Jenkins requires `go`, Docker, Helm 3, Trivy, and Syft on an isolated `target-reconciliation-service-build` agent. Its Docker access is highly privileged and must not be shared with untrusted workloads.


## Selected observability versions

Prometheus 3.13.5 is selected from the current LTS line listed by the official download page (https://prometheus.io/download/). Grafana OSS 13.3.0 is listed by its official download page (https://grafana.com/grafana/download/13.3.0?edition=oss, released 2026-10-27). These are explicit compatibility selections, not immutable image references or claims that image scans passed. Resolved image digests and runtime validation remain release checks. Grafana is an external AGPLv3 component; its license is independent of this repository's Apache-2.0 original source.
