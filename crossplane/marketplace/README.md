# Marketplace assets

`icons/icon.svg` is the official square Archestra icon, copied unchanged from
https://github.com/archestra-ai/archestra/blob/main/platform/frontend/public/logo-icon.svg.
Upbound requires a square SVG named `icon.svg`:
https://github.com/crossplane/upjet-provider-template/blob/main/extensions/icons/USAGE.md.

The release workflow appends this directory to each package before it is
staged, verifies that both runtime manifests are unchanged, and, after
archestra-ai/release-publisher pushes the package to
`xpkg.upbound.io/archestra/provider-archestra`, waits for the public listing
digest and logo. Repository visibility and publishing policy are managed once
by an Upbound organization owner.

Released tags are immutable, so assets are fixed with the next release.
