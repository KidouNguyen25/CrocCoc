# Code signing policy

CrocCoc uses Apache-2.0; its patched Croc worker uses MIT-licensed upstream code. This policy is proposed for maintainer approval. No trusted certificate or SignPath enrollment has been obtained.

- Build from reviewed source with pinned dependencies, worker SHA-256, quality gates and SBOM. GitHub provenance and SHA-256 establish origin/integrity, not Windows publisher trust.
- Stable releases require trusted Authenticode signatures and timestamps for the host and installer and QA on final bytes. The modified upstream worker needs explicit eligibility confirmation from SignPath. Do not sign it until a reviewed build order preserves its embedded SHA-256 pin. Never disable the pin.
- Maintainers must designate authors, reviewers and signing approvers, use MFA, and manually approve signing requests. Never commit signing credentials.
- Sign the host first, regenerate package manifest/checksums, build installer, sign installer, verify trust/timestamps and regenerate final download hashes. Do not reuse pre-signing hashes or attestations.
- Croc encrypts transfer content. The public relay is third-party infrastructure that can observe network metadata. See docs/threat-model.md and docs/release-readme.txt.

Apply at https://signpath.org/; review https://signpath.org/terms. Confirm repository visibility, previous release eligibility and treatment of the modified worker with the provider. Do not claim endorsement until approved.
