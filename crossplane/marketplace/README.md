# Marketplace assets

`icons/icon.png` is the Archestra logo used by the Terraform Registry, copied from
https://avatars.githubusercontent.com/u/216022683?v=4.

The Marketplace publication workflow appends this directory to each released
Crossplane package using `up xpkg append`. It writes to
`xpkg.upbound.io/archestra-ai/provider-archestra` and preserves the original
`xpkg.upbound.io/archestra/provider-archestra` release and its runtime manifests.
