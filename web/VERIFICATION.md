# TRS console verification — 2026-11-12

Project: TargetReconciliationService (TRS), console package
`target-reconciliation-service-console`, version `0.2.0-dev`.
Environment: Linux x86-64, Node v24.22.1, npm 11.9.0.

| Command / check | Result | Evidence from this execution |
| --- | --- | --- |
| `npm ci --no-audit --no-fund` | Passed, 55 packages | `../evidence/trs-web-install.txt` |
| Package metadata against lockfile | Name and version match in both lockfile metadata records | Checked with JSON parser |
| `npm test` | Passed, 7 tests | `../evidence/trs-web-test.txt` |
| `npm run typecheck` | Passed | `../evidence/trs-web-typecheck.txt` |
| `NEXT_TELEMETRY_DISABLED=1 npm run build` | Passed | `../evidence/trs-web-build.txt` |
| `npm audit --omit=dev --json` | Zero reported production vulnerabilities at execution time | `../evidence/trs-web-audit.json` |
| Production server `/`, unauthenticated API, POST API | HTTP 200, 401, 405 respectively; new project title in HTML | `../evidence/trs-web-http.txt` |
| Active source/config search for prior brand and environment prefix | No matches | Source inspected excluding generated dependency/build artifacts |
| Browser visual / responsive inspection | Not run in this execution | No screenshot or accessibility conformance claim |
| Go + PostgreSQL + Kubernetes UI sequence | Not run by console subtask | Consult root integration evidence |

Tests include a rolling-update surge regression: old-revision availability plus
new-revision updated replicas cannot be combined into a false healthy result.
Healthy workloads must have total replicas equal to desired replicas, updated
replicas equal to desired replicas, sufficient availability, and an observed
current generation. Snapshots lacking total replica evidence are rejected.

The prior 2026-11-11 console report and logs are preserved unchanged under
`../evidence/history/web/`; they describe the earlier project name and are not
proof of this renamed release. Test fixtures are synthetic inputs to unit tests,
not live console telemetry. The UI continues to display actual API snapshots only.
