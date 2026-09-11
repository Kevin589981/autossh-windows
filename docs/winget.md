# WinGet publication research (not submitted)

This document records the preparation path for publishing `autossh-windows` to
the WinGet Community Repository. No manifest or pull request has been submitted
as part of this work.

## Recommended package shape

- **PackageIdentifier:** `Kevin589981.autossh-windows` (confirm availability
  with `winget search` and the repository search before authoring)
- **PackageVersion:** the release tag without the leading `v`, for example
  `0.2.0`
- **InstallerType:** `portable`, because the release is a standalone PE
  executable and does not install files or registry entries
- **Architecture:** `x64` for the current `autossh-windows-amd64.exe` asset
- **InstallerUrl:** a version-specific GitHub Release URL, for example
  `https://github.com/Kevin589981/autossh-windows/releases/download/v0.2.0/autossh-windows-amd64.exe`
- **InstallerSha256:** SHA-256 of that exact release asset, calculated with
  `winget hash <path-to-installer>`
- **PackageUrl:** the GitHub repository or release page, so the installer
  domain is discoverable from the publisher's official project page

If ARM64 or additional installer formats are released later, add them as
separate installer entries in the same version manifest rather than reusing an
amd64 URL.

## Official submission workflow

1. Publish a stable, version-specific GitHub Release asset over HTTPS.
2. Confirm the package is not already present and that no PR exists for the
   same version.
3. Generate a multi-file manifest set (version, defaultLocale, installer).
   The community repository does not accept singleton manifests.
4. Use the current supported manifest schema and include the YAML schema header
   in every file. The repository currently documents schema `1.12.0` and newer
   versions in its manifest documentation; use the version requested by the PR
   template at submission time.
5. Validate locally with `winget validate --manifest <path>` and install-test
   with `winget install --manifest <path>` (or Windows Sandbox).
6. Fork `microsoft/winget-pkgs`, create a branch, and open a PR containing
   exactly one package version and manifest files only. Documentation or tooling
   changes must be separate PRs.
7. Complete the Microsoft CLA if prompted and monitor the validation checks.
   The automated pipeline checks schema, URL reachability, domain policy,
   malware/PUA scans, hash, unattended installation, metadata, and catalog
   consistency before moderator review.

## Project-specific blockers to resolve first

- The current release is a portable executable, so WinGet's portable installer
  behavior should be tested on a clean Windows machine. In particular, verify
  that the executable is discoverable on `PATH` after installation and that
  uninstall/upgrade behavior is acceptable for a portable package.
- GitHub Release URLs must remain immutable per version. Do not use a
  `latest/download` URL because replacing the binary would invalidate the
  manifest hash.
- The manifest publisher/name should match the executable's public metadata and
  repository identity. Add stable version and product metadata to the PE file
  if WinGet validation or catalog correlation requires it.
- The current workflow publishes x64 only. Decide whether an ARM64 asset is
  needed before the first manifest so later architecture additions follow the
  repository's update conventions.

## Primary sources

- [Microsoft Learn: Submit packages to Windows Package Manager](https://learn.microsoft.com/en-us/windows/package-manager/package/)
- [winget-pkgs: Authoring manifests](https://github.com/microsoft/winget-pkgs/blob/master/doc/Authoring.md)
- [winget-pkgs: First-time contributor checklist](https://github.com/microsoft/winget-pkgs/blob/master/doc/FirstContribution.md)
- [winget-pkgs: Manifest validation](https://github.com/microsoft/winget-pkgs/blob/master/doc/Validation.md)
- [winget-pkgs: Community repository policies](https://github.com/microsoft/winget-pkgs/blob/master/doc/Policies.md)
- [Microsoft winget-create](https://github.com/microsoft/winget-create)
