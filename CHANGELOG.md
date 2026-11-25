# Changelog

## 0.1.0 (release candidate; not yet stable)

- Windows desktop interface for sending files and directories via the Croc v11.6.4 protocol.
- Receive by connection code, review the incoming manifest before accepting, and choose a local NTFS destination.
- Prevent overwrites, reject unsafe paths, verify content before publication, and support cancellation.
- Default public relay with an optional fail-closed custom relay configuration.
- Vietnamese UI, protected code-copying and progress/status updates.
- Portable package and installer candidate with pinned worker, licenses, SBOM and SHA-256 checksums.

Known limits: one transfer at a time, Windows 10/11 x64 with WebView2, public relay availability and network restrictions, no resume or LAN discovery. Release candidate is unsigned until the signing gate is approved. See docs/release-checklist.md for outstanding QA.
