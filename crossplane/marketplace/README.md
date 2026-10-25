# Marketplace assets

`icons/icon.svg` is the official square Archestra icon, copied unchanged from
https://github.com/archestra-ai/archestra/blob/main/platform/frontend/public/logo-icon.svg.
Upbound requires a square SVG named `icon.svg`:
https://github.com/crossplane/upjet-provider-template/blob/main/extensions/icons/USAGE.md.

The existing Crossplane release job appends this directory to each released
`xpkg.upbound.io/archestra/provider-archestra` package, verifies that both runtime
manifests are unchanged, then waits for the public listing digest and logo.
Repository visibility and publishing policy are managed once by an Upbound
organization owner; release credentials only need the existing registry access.

To repair assets on an existing stable release without rebuilding or publishing
Terraform binaries, run the Release Please workflow with `marketplace_version`
set to its tag (for example `v3.0.3`). Leave the input empty for normal release
processing. Asset repair does not change either architecture's runtime manifest.
