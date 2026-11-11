# Research and dependency decisions

Research date:2026-11-11. Official upstream documentation was consulted; pinned versions are compatibility selections, not claims every component is latest or free of vulnerabilities.

- Go release metadata: https://go.dev/dl/?mode=json and https://go.dev/doc/devel/release . Local initial tooling1.25.5 and subsequent1.27.4 archives were checked against official metadata SHA256. Upstream controller-runtime requires Go 1.25; this project now requires Go 1.27.4 after dependency/security review. Final verification reports the actual toolchain used.
- controller-runtime v0.23.3: https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.23.3 and https://github.com/kubernetes-sigs/controller-runtime/blob/v0.23.3/go.mod . Its module requires Go 1.25 and Kubernetes 0.35 libraries. Direct Kubernetes libraries are pinned0.35.2; real API tests use1.35.0.
- Kubernetes JSON patch: https://kubernetes.io/docs/reference/using-api/api-concepts/ . ResourceVersion and JSON Patch tests reject stale writes at the API server; unit fake-client behavior is not sufficient proof of server semantics.
- PostgreSQL row locks: https://www.postgresql.org/docs/current/explicit-locking.html . One locked aggregate transaction is used initially; SKIP LOCKED is reserved for future queue work, not general state reads.
- NATS consumer acknowledgement/retry semantics: https://docs.nats.io/nats-concepts/jetstream/consumers . Deferred outbox/consumer design must use bounded retries and explicit acknowledgements; exhausted deliveries do not automatically constitute a dead-letter queue.
- Next/React/Tailwind exact package selections and installed lockfile are recorded in web/package.json and web/package-lock.json; build/audit results are in web/VERIFICATION.md.

## Brand check

Exact-name web searches for `ResiliencePlane` and `ResiliencePlane github` found no clear matching project in this session. GitHub's unauthenticated repository-search page could not be fetched, so a complete repository, package, trademark and domain check was not possible. The name remains provisional. No global uniqueness or legal availability claim is made. Recheck GitHub repositories, registries and relevant marks before public release.

## Security-driven dependency revisions

Initial binary-mode scanning identified old Go standard-library advisories and vulnerable symbols in the initial dependency selections. The final manifest requires Go 1.27.4, pgx/v5 v5.9.2, x/net v0.62.0 and x/text v0.43.0 (with compatible MVS transitive updates). Tool evidence includes the exact final build module list and scan outcome; do not interpret older historical scans as the final manifest. Relevant upstream advisory records included https://pkg.go.dev/vuln/GO-2026-5004 and https://pkg.go.dev/vuln/GO-2026-6866 . No assertion is made that these were exploitable in this project's configured query or HTTP usage; vulnerable dependency selections were updated regardless.
