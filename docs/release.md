# Public release checklist

The current repository is a private prepared source artifact,0.1.0-dev. No remote repository, public package, image or tag was created.

Before publishing: recheck provisional name; review Apache-2.0 original source and all dependency/container/optional-component licenses; inspect secrets and build outputs; execute the fresh Docker/Kind recovery scenario; run full Jenkins chain with credentials scoped outside source; scan dependencies/images, produce reviewed SBOM/provenance, sign immutable image digest; verify install/upgrade/teardown and supported platforms; update known limitations, changelog and machine status; review branch protection and security-reporting channel.

Proposed publication steps, only after explicit authorization:

1. Review source archive and verification report locally; create a private or public GitHub repository with the selected name and visibility.
2. Push the inspected main branch and enable required review/status checks. Keep CI on trusted isolated agents with restricted credentials.
3. Configure approved registry and GitOps repository. Publish only the reviewed scanned digest; production credentials never enter .env/Git or untrusted PR jobs.
4. Run and record the complete delivery scenario, then create a reviewed release candidate tag and GitHub release with artifacts, SBOM, signatures and truthful limitations.

A passing source-level test suite does not establish production readiness or complete dependency license approval. Do not label this release enterprise-ready or claim demonstrated large-scale capacity.

The source ZIP includes `repository.bundle` with local history. To restore that history without contacting GitHub, run `git clone /path/to/extracted/resilienceplane/repository.bundle resilienceplane-checkout`. The extracted source tree is also directly buildable. Creating any remote repository or pushing still requires explicit authorization.
