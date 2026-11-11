# ADR0003: Local identity first, outbound agent later
Status: accepted initial boundary; production identity and agent deferred.

Management methods assign operator/viewer identities from private independently generated tokens. Observation is internal controller input, never an untrusted public telemetry API. The initial manager uses current kubeconfig for local development or a namespace-scoped in-cluster service account.

Next architecture requires OIDC issuer/audience/signature validation and per-tenant authorization, outbound mTLS agent sessions, enrolled cluster identities and revoked/rotated credentials. Agents must not send their own authorization decisions. No unrestricted kubeconfigs may be distributed. A versioned protocol, bounded replay buffer and command lease/fencing must be implemented before calling the agent complete.
