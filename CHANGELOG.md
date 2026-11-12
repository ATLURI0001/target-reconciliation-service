# Changelog

## 0.2.0-dev — 2026-11-12

- Rename to TargetReconciliationService (TRS), including Go module, executables, chart, image, API group, annotations, configuration, metrics and console.
- Add push/PR GitHub Actions build/test/lint/Helm and Docker-backed live-lab workflow; hosted execution remains pending publication.
- Add generation-aware ReliabilityTarget status conditions, status subresource/RBAC, and condition transition tests.
- Add repeatable local lab startup/demo/teardown and evidence capture; actual Docker/Kind execution remains subject to documented host capabilities.
- Correct demo selector mismatch and false healthy detection when an old replica masks a failed new rollout.
- Preserve 0.1 evidence verbatim under evidence/history; document breaking development migration boundaries.

## 0.1.0-dev — 2026-11-11

Initial experimental PostgreSQL-backed policy/approval/recovery workflow, local Kubernetes bridge, API/CLI and read-only console. No public or production release has been published.
