# CrocCoc 0.1.0 release gate

Audit baseline main df8271a (2026-11-24). Previous PASS evidence in verification.md is not inherited by the candidate.

| Gate | Required evidence | State |
|---|---|---|
| RC-01 | Clean SHA, version, CI run, frontend/Go/worker tests, vet, dependency scans, SBOM/license | Pending current candidate |
| RC-02 | Installer and portable ZIP, matching worker pin, SHA-256 | Implementation pending execution |
| RC-03 | Two Windows PCs on separate networks, real relay, small/large/multiple files, Unicode/empty directories, SHA-256 compare | NOT RUN |
| RC-04 | Consent before payload, collisions/reparse, cancel/network failure, process cleanup, reboot/staging recovery | NOT RUN for final package |
| RC-05 | Clipboard history/cloud clipboard on two machines with positive control | NOT RUN |
| RC-06 | Native UI, keyboard/screen reader, mockup regression and errors | Pending QA |
| RC-07 | Clean Win10/11 x64, missing WebView2, install/upgrade/uninstall, Defender/SmartScreen/firewall | NOT RUN |
| RC-08 | SignPath application, trusted Authenticode/timestamp, approved worker treatment, final manifest/checksums | BLOCKED on provider/owner |
| RC-09 | Re-test final signed package by exact hash, sign-off | NOT RUN |
| RC-10 | GitHub stable tag/release, assets/notes, landing download, support/privacy links | Pending |

Run the Windows release candidate Actions workflow from reviewed source with version 0.1.0-rc.1. It creates unsigned installer and portable artifacts, checksums and (public repo) provenance, without publishing. For local builds run scripts/build-release.ps1 then scripts/build-installer.ps1 with matching PackageVersion. Inno Setup 6 is required. Missing WebView2 stops installation and directs interactive users to the official Microsoft page. Install Evergreen Bootstrapper online or Standalone offline, then retry.

Before applying to SignPath, confirm prior release requirement and permitted treatment of the patched worker. If a public unsigned prerelease is required, label it experimental and unsigned, disclose SmartScreen warnings, verify checksums/provenance, and obtain owner approval first. Follow CODE_SIGNING.md for signing order.

QA records exact ZIP/installer SHA-256, Git SHA, Windows/WebView2 versions, relay route, fixture result, PASS/FAIL/BLOCKED/NOT RUN and tester. Never record codes/passwords or personal paths. Use e2e-runbook.md and verification.md for criteria. Retest the signed package; do not reuse evidence from different binaries. Publication requires owner sign-off.
