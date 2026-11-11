# ADR0002: Test-only opt-in rollback with compare-and-swap
Status: accepted for first slice, 2026-11-11.

Restarting arbitrary failed pods duplicates native healing. Instead demonstrate an explicitly authorized image rollback of an opted-in test Deployment with a trusted operator-declared digest. Reject recognized Argo CD and Flux ownership to avoid controller conflict. Source spec generation must match approval; current resourceVersion is the atomic patch fence. Persist the execution marker in the same patch as the image.

Consequence: a generic observed health counter cannot prove application health. First verification is limited to rollout readiness; request-level and SLO checks are later requirements. The local demo captures the digest from a workload that actually became Ready. No stateful failover, routing or cloud mutation is implemented.
