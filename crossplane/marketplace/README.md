# Marketplace assets

`icons/icon.png` is the Archestra logo used by the Terraform Registry, copied from
https://avatars.githubusercontent.com/u/216022683?v=4.

The Crossplane release job appends this directory to each released package
using `up xpkg append`. It adds assets to the existing
`xpkg.upbound.io/archestra/provider-archestra` release and preserves its runtime
manifests.
