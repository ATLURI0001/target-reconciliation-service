# Console verification — 2026-11-11

Environment: Linux x86-64, Node v24.22.1, npm 11.9.0.

| Command / check | Result | Evidence |
| --- | --- | --- |
| `npm ci --ignore-scripts --no-audit --no-fund` | Passed | 55 packages installed from `package-lock.json` |
| `npm test` | Passed, 5 tests | `test-output.txt` |
| `npm run typecheck` | Passed | `typecheck-output.txt` |
| Production build (`NEXT_TELEMETRY_DISABLED=1 web/node_modules/.bin/next build web`, from repository root) | Passed after final audit-limit wording change | `build-output.txt` |
| `npm audit --omit=dev --json` | No reported production dependency vulnerabilities | `audit-production.json`; registry result at execution time, not a security guarantee |
| Production server `GET /` | HTTP 200 and expected empty-state HTML | `http-smoke.json` |
| Unauthenticated `GET /api/snapshot` | HTTP 401 | `http-smoke.json` |
| `POST /api/snapshot` | HTTP 405 | `http-smoke.json` |
| Browser visual / responsive inspection | Blocked | Chromium could not launch: process-singleton socket creation returned `Operation not permitted` |
| Live Go + PostgreSQL + Kubernetes UI sequence | Not executed by console subtask | Root verification report determines backend integration status |

The test runner uses controlled fixture values to test freshness and health
calculations. Fixtures are not displayed as live operational telemetry. No
screenshots or generated demonstration metrics are included.

A repeat through `npm run build` encountered `ENOSPC` while npm wrote to the
full `/home/agent` tmpfs. Invoking the identical installed Next build CLI directly
completed successfully; no dependency source files were removed.
