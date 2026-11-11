# Contributing

Start with README.md, ARCHITECTURE.md, and the implementation status. Prefer one verifiable vertical change per pull request. Discuss broad architectural or privilege changes in an issue first.

Use Go 1.25 or newer, run `make lint`, `make race`, and the integration checks relevant to the change. Record actual commands and failures. Kubernetes fake-client tests do not replace a real cluster demonstration. Do not claim production scale or integration success without evidence.

Never submit credentials, tokens, kubeconfigs, Terraform state, or logs containing secrets. Add tests for authorization, policy, execution, and persistence changes. Preserve license notices and list new third-party material in NOTICE where required.

Significant behavior changes need documentation, status updates, and an architecture decision record. The project uses ordinary feature branches and reviewed pull requests; no remote publication is performed by local setup.
