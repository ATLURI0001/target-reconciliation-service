# Management interface

Run the Go API with PostgreSQL; see local-lab.md. `api/openapi.yaml` describes the first endpoint set; it is not a generated comprehensive schema of every observation/plan field. Go domain types in internal/core/model.go are authoritative for stored field shapes. Production API compatibility/version migration is still an alpha concern.

Use `Authorization: Bearer <private token>` over loopback for the local lab. Server-selected local-operator can register, change policy, approve and cancel. local-viewer can read. All other credentials are rejected. No request can supply its own role/subject. Bodies reject unknown fields, invalid JSON and more than one JSON value, with a16KiB limit.

`GET /v1/snapshot` returns current clusters/observations/incidents, policy and the last 200 audit entries. `GET /v1/audit?cursor=0&limit=100` returns ascending `items` and `nextCursor`; request the next page using that sequence. Invalid limits/cursors return400. Full aggregate reads remain a scale limitation even when the response is paged.

`POST /v1/clusters` registers a DNS-label id and environment `test`; health remains unobserved until the trusted controller reads Kubernetes. `PUT /v1/policy` replaces the complete policy. `/v1/incidents/{id}/approve` and `/cancel` accept POST. Duplicate or terminal approvals return409; execution admission is keyed independently. The dry-run approval returns200 with a DryRun state, never an execution. Inspect snapshot or audit for the decision.

Policy defaults: namespace rp-demo, maxReplicas 3, maxConcurrent 1, freshness 30s, continuous fresh-failure detection 30s, dryRun=true, emergencyStop=false. A rollback needs a distinct digest-pinned recovery image, opt-in, exclusive runtime ownership, fresh evidence and explicit approval. Even an operator cannot bypass policy. Policy validation rejects protected namespaces, invalid freshness/concurrency and detection durations outside1..86400 seconds.

Responses:400 invalid request,401 invalid/missing authentication,403 known principal/policy denied,404 missing entity,409 invalid transition,429 admission-rate limit,503 queue/store unavailable,500 unexpected internal failure with no raw database error. There is a global burst 100/20rps limiter and 32 in-flight request cap. A denied authenticated mutation is committed to audit before403 is returned.

## CLI

```sh
make build
set -a; . ./.env; set +a
export RP_TOKEN="$RP_VIEWER_TOKEN"
./bin/rpctl snapshot
./bin/rpctl audit
# For an explicitly selected incident, with operator authority:
export RP_TOKEN="$RP_OPERATOR_TOKEN"
./bin/rpctl approve INCIDENT_ID
./bin/rpctl cancel INCIDENT_ID
```

`RP_API_URL` defaults to http://127.0.0.1:8080. The CLI refuses plaintext HTTP outside localhost and refuses redirects. It never prints its bearer token. `rpctl register local` and `rpctl policy path/to/policy.json` are operator commands. CLI `audit` currently returns the first 100 entries; use HTTP pagination for later entries. The read-only console uses its own server-configured backend URL and never persists the browser token.
