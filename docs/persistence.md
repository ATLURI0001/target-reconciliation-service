# Persistence contract

The executable requires PostgreSQL. The memory repository exists only for unit
and workflow tests and is never selected as a production fallback.

The initial store uses a single JSONB aggregate (`rp_state`) containing cluster
records, observations, policy, incidents and the ordered audit timeline. Each
mutation acquires `SELECT ... FOR UPDATE` on that row under READ COMMITTED,
computes a change, appends audit rows, writes a new aggregate revision, and
commits atomically. Views see a committed snapshot. No Kubernetes or network I/O
belongs inside a transaction callback. The aggregate row serializes API and
controller decisions across processes; callbacks must not re-enter a store.

`rp_audit` is a separately queryable append-only mirror. Mutation callbacks
cannot change committed audit entries. Database triggers reject UPDATE, DELETE
and TRUNCATE on audit rows, including accidental owner operations. The database
owner can still disable triggers or alter tables; this is not tamper-proof
storage against a database administrator. No audit cryptographic integrity or
external immutable retention is claimed.

Schema bootstrap is embedded and transactionally protected by an advisory lock.
Version 1 uses one schema-owning database identity for local reproducibility.
For a hardened deployment, run migrations separately, use a non-owner runtime
role with SELECT/UPDATE on `rp_state`, INSERT/SELECT on `rp_audit`, revoke schema
CREATE and audit UPDATE/DELETE/TRUNCATE, and ship audit copies to external
retention. Separate migration tooling and runtime permissions remain planned.
Require TLS (`sslmode=verify-full`) for network databases. The Compose lab uses
an isolated bridge network and a non-public database port.

Limits: every update rewrites the aggregate and audit history, every API view
reads it, and a single row limits write concurrency. This design supports an
inspectable laboratory prototype, not large-scale inventory ingestion. Migrate
to normalized cluster/target/incident tables with per-target row locks,
transactional outbox, partitioned audit storage, retention and indexed paginated
queries before scale validation. No throughput or durability claim beyond
PostgreSQL's configured guarantees is implied.

Run `go test -race ./internal/store` for memory correctness. Run with
`TEST_DATABASE_URL=... go test -race ./internal/store` against a disposable
PostgreSQL database for integration coverage. Tests add uniquely named cluster
records and audit entries; they do not erase existing records. Do not target a
production database. Skipped integration tests are not successful validation of
persistence. Database process-loss, backup/restore and storage exhaustion require additional resilience tests.
Concurrent writes across two independently connected pools are covered by the
integration test; subprocess fault injection remains unverified.
