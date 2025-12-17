# Changelog

## [0.2.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.1.0...v0.2.0) (2025-12-17)


### Features

* add `archestra_chat_llm_provider_api_key` resource ([#43](https://github.com/archestra-ai/terraform-provider-archestra/issues/43)) ([cefcfca](https://github.com/archestra-ai/terraform-provider-archestra/commit/cefcfcae3c7ae4e9fcb37cdc8159c6d9c2608776))

## [0.1.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.0.5...v0.1.0) (2025-12-17)


### Features

* add `archestra_mcp_server` Resource ([#15](https://github.com/archestra-ai/terraform-provider-archestra/issues/15)) ([8528aba](https://github.com/archestra-ai/terraform-provider-archestra/commit/8528aba32a1f5bf207204f2fad37fe860a591c10))
* add `archestra_organization_settings` resource ([#37](https://github.com/archestra-ai/terraform-provider-archestra/issues/37)) ([d54e0ac](https://github.com/archestra-ai/terraform-provider-archestra/commit/d54e0ac50e207aeac9a935b7b087f0f94b9bff74))
* add `archestra_team_external_group` resource and `archestra_team_external_groups` data source ([#34](https://github.com/archestra-ai/terraform-provider-archestra/issues/34)) ([aa7b286](https://github.com/archestra-ai/terraform-provider-archestra/commit/aa7b2861179bdff8bf1e39ff9fb52731989dd2a5))
* Add cost-saving resources for token pricing, limits, and optimization ([#22](https://github.com/archestra-ai/terraform-provider-archestra/issues/22)) ([8129190](https://github.com/archestra-ai/terraform-provider-archestra/commit/81291907126fdfdc163a91f2821976cf84a078aa))


### Bug Fixes

* add retry mechanism for async tool assignment in agent_tool data source ([#33](https://github.com/archestra-ai/terraform-provider-archestra/issues/33)) ([b41c866](https://github.com/archestra-ai/terraform-provider-archestra/commit/b41c866aeef0bbd62b7120be63c155f48338527a))


### Dependencies

* **terraform:** bump the terraform-go-dependencies group with 2 updates ([#24](https://github.com/archestra-ai/terraform-provider-archestra/issues/24)) ([a9c3e85](https://github.com/archestra-ai/terraform-provider-archestra/commit/a9c3e8556e0335e6a297f8f01580d21e9827cfcd))

## [0.0.5](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.0.4...v0.0.5) (2025-11-01)


### Features

* add `labels` to `archestra_agent` resource ([#12](https://github.com/archestra-ai/terraform-provider-archestra/issues/12)) ([acf2847](https://github.com/archestra-ai/terraform-provider-archestra/commit/acf28476cfbee55cdae551383c60bc4ec9de972e))

## [0.0.4](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.0.3...v0.0.4) (2025-10-27)


### Documentation

* remove `is_demo` and `is_default` from `archestra_agent` example ([147a05e](https://github.com/archestra-ai/terraform-provider-archestra/commit/147a05eb123f36c0f989ba44629dc08b1f1d6202))

## [0.0.3](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.0.2...v0.0.3) (2025-10-27)


### Documentation

* improve/clarify resource argument documentation + remove `is_default` + `is_demo` from agent resource ([#9](https://github.com/archestra-ai/terraform-provider-archestra/issues/9)) ([16fa690](https://github.com/archestra-ai/terraform-provider-archestra/commit/16fa69009ea967376a2a14c2b6dc51dcc3dcec41))

## [0.0.2](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.0.1...v0.0.2) (2025-10-27)


### Bug Fixes

* outstanding provider issues ([#7](https://github.com/archestra-ai/terraform-provider-archestra/issues/7)) ([c33e1ec](https://github.com/archestra-ai/terraform-provider-archestra/commit/c33e1ec1160976dce6434a4866594c066e9d0162))

## 0.0.1 (2025-10-26)


### Features

* Archestra Terraform provider (hello world) ([#1](https://github.com/archestra-ai/terraform-provider-archestra/issues/1)) ([e1ff1e4](https://github.com/archestra-ai/terraform-provider-archestra/commit/e1ff1e482d93bfa4562c0eeb2bcc5d311fe09fae))
