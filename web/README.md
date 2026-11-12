# TargetReconciliationService console

TRS (TargetReconciliationService) is the renamed project. This read-only Next.js / TypeScript / Tailwind console displays the actual Go API
snapshot. It does not contain seeded metrics, simulated clusters, approval buttons,
or unimplemented navigation pages. Cluster inventory, workload health, incidents,
plans, safety policy, and audit history are supported. Capacity metrics, topology,
settings, deployment management, and the full product page set remain planned.

## Run

Prerequisites: Node.js 22.13 or newer, npm, and the running Go API on port 8080.

```sh
cd web
npm ci
npm test
npm run typecheck
npm run build
TRS_API_URL=http://127.0.0.1:8080 npm start
```

On Windows use WSL2; on macOS and Linux use the shell commands above. Open
http://127.0.0.1:3000 and paste the locally generated viewer token from the core
setup. `npm run dev` starts development mode. Both commands bind to loopback.

The browser sends a bearer token to the same-origin `/api/snapshot` endpoint.
The server forwards it only to the configured `TRS_API_URL` origin. The token stays
in React memory: it is never stored in local/session storage, cookies, URLs, or
source configuration. Reload or disconnect to clear it. Use HTTPS if hosting
this console beyond loopback. The initial authentication mode is a local development
facility, not a production OIDC implementation.

The console refreshes every 10 seconds and recalculates freshness every second.
A failed API refresh retains the last snapshot with an error banner; stale telemetry
becomes unobservable. Replica counts are Kubernetes status, not CPU or memory usage. Healthy status requires
the complete desired replica set to be updated; availability from an old revision
during a rolling-update surge does not prove successful recovery.
The response is limited to 4 MB in this initial console. Audit history shows the most
recent 200 events from the snapshot; use the paginated Go API `/v1/audit` endpoint
for the full history. In-console pagination remains planned.

## Checks

`npm test` checks freshness, generation-aware and surge-safe health, active incident counting,
and transport validation with Node's built-in test runner. `npm run typecheck`
and `npm run build` check TypeScript and Next.js compilation. These do not prove
browser accessibility or a live PostgreSQL/Kubernetes workflow. Review the root
verification report for checks actually run in the build environment.
