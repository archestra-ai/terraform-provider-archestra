# Changelog

## [0.4.1](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.4.0...v0.4.1) (2025-12-29)


### Bug Fixes

* address `docker_image` without `command` + env var bugs in `archestra_mcp_registry_catalog_item` resource ([#62](https://github.com/archestra-ai/terraform-provider-archestra/issues/62)) ([4f5c02b](https://github.com/archestra-ai/terraform-provider-archestra/commit/4f5c02be8590af3915cd48d11b7bef995d6fe470))

## [0.4.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.3.0...v0.4.0) (2025-12-29)


### Features

* Add `remote_config` support to `archestra_mcp_registry_catalog_item` resource ([#60](https://github.com/archestra-ai/terraform-provider-archestra/issues/60)) ([836cb78](https://github.com/archestra-ai/terraform-provider-archestra/commit/836cb78d14c1917f00ca944e05a0e46c85576174))

## [0.3.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.2.0...v0.3.0) (2025-12-19)


### Features

* add `archestra_dual_llm_config` resource ([#50](https://github.com/archestra-ai/terraform-provider-archestra/issues/50)) ([9e55ec8](https://github.com/archestra-ai/terraform-provider-archestra/commit/9e55ec860fde2cd4dd14f0d4582d0a30290bb2b6))
* add `archestra_profile_tool` resource + rename `archestra_agent_tool` datasource -&gt; `archestra_profile_tool` ([#47](https://github.com/archestra-ai/terraform-provider-archestra/issues/47)) ([e2345ec](https://github.com/archestra-ai/terraform-provider-archestra/commit/e2345ec0436ae2b12159bb3aba907c55cb687a7d))
* after mcp server installation, wait for tools to be available ([#54](https://github.com/archestra-ai/terraform-provider-archestra/issues/54)) ([2b69232](https://github.com/archestra-ai/terraform-provider-archestra/commit/2b6923253b1bd07c7b7e099856f929e5ac2d1262))
* rename `archestra_mcp_server` resource to `archestra_mcp_registry_catalog_item` ([#46](https://github.com/archestra-ai/terraform-provider-archestra/issues/46)) ([baf01b6](https://github.com/archestra-ai/terraform-provider-archestra/commit/baf01b64bd1b8018379e0d54428f8451aafeb0e7))

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
