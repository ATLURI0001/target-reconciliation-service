# ADR0001: One service and a serialized durable core
Status: accepted for the initial lab milestone, 2026-11-11.

A distributed workflow can lose approval or duplicate an action if state and audit updates diverge. Use a modular monolith and PostgreSQL transaction for each domain change. A JSONB singleton plus append-only audit mirror keeps early invariant checking inspectable; avoid premature services and messaging dependencies.

Consequence: all writers contend, and audit growth increases copy/write cost. This is intentionally unsuitable for claimed large-scale operation. Normalize tables and partition admission before increasing scale. Memory repository is for unit tests only; production executable requires PostgreSQL. SQL callbacks must never perform external I/O.
