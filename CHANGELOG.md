# Changelog

## [3.0.2](https://github.com/archestra-ai/terraform-provider-archestra/compare/v3.0.1...v3.0.2) (2026-10-24)


### Dependencies

* bump the github-actions-dependencies group across 3 directories with 12 updates ([#201](https://github.com/archestra-ai/terraform-provider-archestra/issues/201)) ([671a76e](https://github.com/archestra-ai/terraform-provider-archestra/commit/671a76e021d48e75fe4af60d8ad9be9e70c0dab6))
* **terraform:** bump golang.org/x/tools from 0.43.0 to 0.52.0 in the terraform-go-dependencies group ([#200](https://github.com/archestra-ai/terraform-provider-archestra/issues/200)) ([2eab2c6](https://github.com/archestra-ai/terraform-provider-archestra/commit/2eab2c683769632e550867c522d91be142dac438))


### Code Refactoring

* **ci:** use shared organization authentication actions ([#198](https://github.com/archestra-ai/terraform-provider-archestra/issues/198)) ([2a6a603](https://github.com/archestra-ai/terraform-provider-archestra/commit/2a6a6039edd7f8816c025290b29d1b2121ace4a5))

## [3.0.1](https://github.com/archestra-ai/terraform-provider-archestra/compare/v3.0.0...v3.0.1) (2026-10-22)


### Bug Fixes

* **examples:** make the published examples apply on 3.0.0 and validate them in CI ([#196](https://github.com/archestra-ai/terraform-provider-archestra/issues/196)) ([1a00a30](https://github.com/archestra-ai/terraform-provider-archestra/commit/1a00a30a931eb1cdce9d8ab574280e64c453ccad))

## [3.0.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v2.4.1...v3.0.0) (2026-10-22)


### ⚠ BREAKING CHANGES

* add platform-version.txt and stop sending retired request fields ([#184](https://github.com/archestra-ai/terraform-provider-archestra/issues/184))
* consolidate provider resources and modernize both Crossplane scopes ([#180](https://github.com/archestra-ai/terraform-provider-archestra/issues/180))

### Features

* add platform-version.txt and stop sending retired request fields ([#184](https://github.com/archestra-ai/terraform-provider-archestra/issues/184)) ([11e785b](https://github.com/archestra-ai/terraform-provider-archestra/commit/11e785b2d5384f78a08fff43854b6e6559828020))
* consolidate provider resources and modernize both Crossplane scopes ([#180](https://github.com/archestra-ai/terraform-provider-archestra/issues/180)) ([bf083ca](https://github.com/archestra-ai/terraform-provider-archestra/commit/bf083ca596587ccf8565c78017b7e56ab928f233))


### Bug Fixes

* **ci:** validate Crossplane coverage badges without pushing to main ([#189](https://github.com/archestra-ai/terraform-provider-archestra/issues/189)) ([bf68e86](https://github.com/archestra-ai/terraform-provider-archestra/commit/bf68e86b3718cffbf33168368c1e855a5cb4f3e6))
* disable Archestra telemetry in CI ([#182](https://github.com/archestra-ai/terraform-provider-archestra/issues/182)) ([8c76a03](https://github.com/archestra-ai/terraform-provider-archestra/commit/8c76a03602e260a4b512e2269313ee5ef57cc4ba))
* keep only API inputs and the credential in the virtual key schema ([#191](https://github.com/archestra-ai/terraform-provider-archestra/issues/191)) ([c937a0c](https://github.com/archestra-ai/terraform-provider-archestra/commit/c937a0c9882704760ee006a53e79b330bc893b56))


### Documentation

* correct the 3.0.0 release and compatibility notes ([#190](https://github.com/archestra-ai/terraform-provider-archestra/issues/190)) ([83479ba](https://github.com/archestra-ai/terraform-provider-archestra/commit/83479ba3404d208c2512dd2013ede8496694b6f6))
* make the Vault-vended key example work for service accounts ([#185](https://github.com/archestra-ai/terraform-provider-archestra/issues/185)) ([aff1cee](https://github.com/archestra-ai/terraform-provider-archestra/commit/aff1ceeb895ae5e66f95f7750bee6cce625fe203))

## [2.4.1](https://github.com/archestra-ai/terraform-provider-archestra/compare/v2.4.0...v2.4.1) (2026-10-16)


### Bug Fixes

* bundle matching Terraform release in Crossplane packages ([#177](https://github.com/archestra-ai/terraform-provider-archestra/issues/177)) ([1bfa6de](https://github.com/archestra-ai/terraform-provider-archestra/commit/1bfa6def6404de255b54c9729ba9ac604284fd8f))
* **ci:** use a published credential acceptance image and matching client ([#179](https://github.com/archestra-ai/terraform-provider-archestra/issues/179)) ([4709b57](https://github.com/archestra-ai/terraform-provider-archestra/commit/4709b5714c73e3521c312f721789dcc764856506))

## [2.4.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v2.3.1...v2.4.0) (2026-09-14)


### Features

* manage shared GitHub App user credentials ([#175](https://github.com/archestra-ai/terraform-provider-archestra/issues/175)) ([eb89448](https://github.com/archestra-ai/terraform-provider-archestra/commit/eb8944846a3df92d5845ccea099017fce62b5b34))

## [2.3.1](https://github.com/archestra-ai/terraform-provider-archestra/compare/v2.3.0...v2.3.1) (2026-09-13)


### Bug Fixes

* allow unknown runtime settings during agent validation ([#172](https://github.com/archestra-ai/terraform-provider-archestra/issues/172)) ([ef21250](https://github.com/archestra-ai/terraform-provider-archestra/commit/ef2125030bf088099cfcbe66d3f8e123f15220fc))
* use pinned goimports in Crossplane releases ([#173](https://github.com/archestra-ai/terraform-provider-archestra/issues/173)) ([2706dcc](https://github.com/archestra-ai/terraform-provider-archestra/commit/2706dcc48562c327030c9110fa02c11ca28e208a))

## [2.3.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v2.2.1...v2.3.0) (2026-09-13)


### Features

* **agent:** manage native Claude runtime authentication ([#170](https://github.com/archestra-ai/terraform-provider-archestra/issues/170)) ([cfede50](https://github.com/archestra-ai/terraform-provider-archestra/commit/cfede50778ff3817cff1d791f499e83aa51e6e22))

## [2.2.1](https://github.com/archestra-ai/terraform-provider-archestra/compare/v2.2.0...v2.2.1) (2026-09-03)


### Bug Fixes

* **tool:** support paginated tool lookup ([#168](https://github.com/archestra-ai/terraform-provider-archestra/issues/168)) ([1ba7d8f](https://github.com/archestra-ai/terraform-provider-archestra/commit/1ba7d8f8d8e2afb4618ee68370cbf13bc4b51bb6))

## [2.2.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v2.1.1...v2.2.0) (2026-09-03)


### Features

* **agent:** rename background execution to runtime ([#166](https://github.com/archestra-ai/terraform-provider-archestra/issues/166)) ([e1488c5](https://github.com/archestra-ai/terraform-provider-archestra/commit/e1488c5182f89bd9a7a92688f89bf96e97e1d988))

## [2.1.1](https://github.com/archestra-ai/terraform-provider-archestra/compare/v2.1.0...v2.1.1) (2026-09-01)


### Bug Fixes

* **agent:** tolerate unknown background_execution environment/credentials lists ([#164](https://github.com/archestra-ai/terraform-provider-archestra/issues/164)) ([2a7a1e2](https://github.com/archestra-ai/terraform-provider-archestra/commit/2a7a1e2fea4b1d5e34e4209ca8e37c42b6046ec0))

## [2.1.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v2.0.0...v2.1.0) (2026-09-01)


### Features

* agent background_execution, execution credentials, and tool exclusions ([#163](https://github.com/archestra-ai/terraform-provider-archestra/issues/163)) ([8c50ba4](https://github.com/archestra-ai/terraform-provider-archestra/commit/8c50ba472daf2e7fd3e177c51ebd24899c5ebd43))


### Dependencies

* **terraform:** bump github.com/oapi-codegen/runtime from 1.6.0 to 1.7.0 in the terraform-go-dependencies group across 1 directory ([#159](https://github.com/archestra-ai/terraform-provider-archestra/issues/159)) ([11482a2](https://github.com/archestra-ai/terraform-provider-archestra/commit/11482a212ce6f0397b5700501d7f15d378758c95))

## [2.0.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v1.5.0...v2.0.0) (2026-08-27)


### ⚠ BREAKING CHANGES

* remove archestra_llm_proxy ([#160](https://github.com/archestra-ai/terraform-provider-archestra/issues/160))

### Features

* remove archestra_llm_proxy ([#160](https://github.com/archestra-ai/terraform-provider-archestra/issues/160)) ([a716c8a](https://github.com/archestra-ai/terraform-provider-archestra/commit/a716c8ac71ee9d2107c7d322753bab1dd598435c))

## [1.5.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v1.4.0...v1.5.0) (2026-08-20)


### Features

* scope on archestra_mcp_server_installation ([#157](https://github.com/archestra-ai/terraform-provider-archestra/issues/157)) ([771a6f3](https://github.com/archestra-ai/terraform-provider-archestra/commit/771a6f338e1bb533886dbf4dc30b9ecaa5fdeb59))

## [1.4.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v1.3.0...v1.4.0) (2026-08-15)


### Features

* tool_exposure_mode on archestra_mcp_gateway ([#155](https://github.com/archestra-ai/terraform-provider-archestra/issues/155)) ([eb4f512](https://github.com/archestra-ai/terraform-provider-archestra/commit/eb4f512156dec31eb9dbb9871285605cd1740570))

## [1.3.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v1.2.0...v1.3.0) (2026-08-13)


### Features

* archestra_agent_delegation resource ([#154](https://github.com/archestra-ai/terraform-provider-archestra/issues/154)) ([aa21a0c](https://github.com/archestra-ai/terraform-provider-archestra/commit/aa21a0c54a18303c8457dc82f4738321912ed3e5))


### Dependencies

* **terraform:** bump github.com/hashicorp/terraform-plugin-sdk/v2 from 2.40.0 to 2.40.1 in the terraform-go-dependencies group across 1 directory ([#91](https://github.com/archestra-ai/terraform-provider-archestra/issues/91)) ([7c076e6](https://github.com/archestra-ai/terraform-provider-archestra/commit/7c076e67eb67f289b7d5b514e97ccd177d5f4f0a))
* **terraform:** bump github.com/oapi-codegen/runtime from 1.4.0 to 1.4.1 in the terraform-go-dependencies group ([#151](https://github.com/archestra-ai/terraform-provider-archestra/issues/151)) ([4bb658e](https://github.com/archestra-ai/terraform-provider-archestra/commit/4bb658ed1d579c11c99b9dcaede08804eef68907))
* **terraform:** bump the terraform-go-dependencies group across 1 directory with 2 updates ([#153](https://github.com/archestra-ai/terraform-provider-archestra/issues/153)) ([4879d5a](https://github.com/archestra-ai/terraform-provider-archestra/commit/4879d5aac7738ab9b9cd9f91d563c2de18944ede))

## [1.2.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v1.1.4...v1.2.0) (2026-05-22)


### Features

* **ci:** add Crossplane coverage report + README badge ([#143](https://github.com/archestra-ai/terraform-provider-archestra/issues/143)) ([c2fa4df](https://github.com/archestra-ai/terraform-provider-archestra/commit/c2fa4df840812266d85df476bc96964fe39ab89b))


### Bug Fixes

* **provider:** treat empty ID in Read as "not yet created" ([#147](https://github.com/archestra-ai/terraform-provider-archestra/issues/147)) ([1a05b82](https://github.com/archestra-ai/terraform-provider-archestra/commit/1a05b824c7abba1b75551b587f5f18929b077bcd))


### Documentation

* add Crossplane MR quickstart to README step 5 ([#149](https://github.com/archestra-ai/terraform-provider-archestra/issues/149)) ([2faf041](https://github.com/archestra-ai/terraform-provider-archestra/commit/2faf0419b487e8a39371caac4e7df13f6448bbba))
* add Crossplane section + tighten Releases explanation ([#145](https://github.com/archestra-ai/terraform-provider-archestra/issues/145)) ([7f1fdba](https://github.com/archestra-ai/terraform-provider-archestra/commit/7f1fdba5c09e914a93007da0ee849a4038ce4e2c))
* add Resource coverage table to README ([#148](https://github.com/archestra-ai/terraform-provider-archestra/issues/148)) ([d2d28b1](https://github.com/archestra-ai/terraform-provider-archestra/commit/d2d28b1b7a1f3cbf22b4ab92d3bd23b6fdc6958f))

## [1.1.4](https://github.com/archestra-ai/terraform-provider-archestra/compare/v1.1.3...v1.1.4) (2026-05-22)


### Bug Fixes

* **ci:** build both linux archs before pushing the xpkg ([#141](https://github.com/archestra-ai/terraform-provider-archestra/issues/141)) ([2271b98](https://github.com/archestra-ai/terraform-provider-archestra/commit/2271b98d58f86ff4b0ebfc8f51030b3a0874e5d1))

## [1.1.3](https://github.com/archestra-ai/terraform-provider-archestra/compare/v1.1.2...v1.1.3) (2026-05-22)


### Bug Fixes

* **ci:** read release-please outputs without .-- namespace prefix ([#139](https://github.com/archestra-ai/terraform-provider-archestra/issues/139)) ([29afd33](https://github.com/archestra-ai/terraform-provider-archestra/commit/29afd3317a8df25de752dd90c730d979b25a76d4))

## [1.1.2](https://github.com/archestra-ai/terraform-provider-archestra/compare/v1.1.1...v1.1.2) (2026-05-22)


### Bug Fixes

* **ci:** remove trailing comma from release-please manifest ([#137](https://github.com/archestra-ai/terraform-provider-archestra/issues/137)) ([7b13089](https://github.com/archestra-ai/terraform-provider-archestra/commit/7b13089ae23b78bb366d138b5e2d6e4490d201c8))

## [1.1.1](https://github.com/archestra-ai/terraform-provider-archestra/compare/v1.1.0...v1.1.1) (2026-05-22)


### Bug Fixes

* **ci:** authenticate to Upbound via docker login, not up login ([#131](https://github.com/archestra-ai/terraform-provider-archestra/issues/131)) ([7c27a46](https://github.com/archestra-ai/terraform-provider-archestra/commit/7c27a469789a34ffc7eabb4da51eb0d2f07e9ec7))
* **crossplane:** point install example at the real registry ([#133](https://github.com/archestra-ai/terraform-provider-archestra/issues/133)) ([b023c57](https://github.com/archestra-ai/terraform-provider-archestra/commit/b023c570f24af2896f75adf71e2c20fb47c87422))

## [1.1.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v1.0.0...v1.1.0) (2026-05-22)


### Features

* **crossplane:** seed initial release at v1.0.0 ([#129](https://github.com/archestra-ai/terraform-provider-archestra/issues/129)) ([e31468e](https://github.com/archestra-ai/terraform-provider-archestra/commit/e31468e50ecd6971b5ddeff418d885d081ad78e7))


### Bug Fixes

* **ci:** restore missing comma in release-please manifest ([#127](https://github.com/archestra-ai/terraform-provider-archestra/issues/127)) ([8ee2d5b](https://github.com/archestra-ai/terraform-provider-archestra/commit/8ee2d5b7def968d4f071756232fa83ad06874d37))

## [1.0.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.6.0...v1.0.0) (2026-05-06)


### ⚠ BREAKING CHANGES

* merge-patch architecture, 11 bug fixes, UX hardening (v1.2.20) ([#87](https://github.com/archestra-ai/terraform-provider-archestra/issues/87))

### Features

* merge-patch architecture, 11 bug fixes, UX hardening (v1.2.20) ([#87](https://github.com/archestra-ai/terraform-provider-archestra/issues/87)) ([a263c9c](https://github.com/archestra-ai/terraform-provider-archestra/commit/a263c9cfe08cb34e06e618e08c8e28f684cb534b))


### Bug Fixes

* **test:** silence staticcheck SA5011 after t.Fatal nil-check ([#90](https://github.com/archestra-ai/terraform-provider-archestra/issues/90)) ([272017c](https://github.com/archestra-ai/terraform-provider-archestra/commit/272017cea2d2e2e8e2632c96c81308473e9da005))


### Dependencies

* **terraform:** bump the terraform-go-dependencies group across 1 directory with 2 updates ([#89](https://github.com/archestra-ai/terraform-provider-archestra/issues/89)) ([7e5d841](https://github.com/archestra-ai/terraform-provider-archestra/commit/7e5d841508cc11ba3512556d568d0df202afe91e))
* **terraform:** bump the terraform-go-dependencies group across 1 directory with 5 updates ([#80](https://github.com/archestra-ai/terraform-provider-archestra/issues/80)) ([39f2051](https://github.com/archestra-ai/terraform-provider-archestra/commit/39f205199e902d8288ab7e45f067989b0676d7ea))

## [0.6.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.5.0...v0.6.0) (2026-04-23)


### Features

* update provider for Archestra v1.2.20 API ([#82](https://github.com/archestra-ai/terraform-provider-archestra/issues/82)) ([f658cd4](https://github.com/archestra-ai/terraform-provider-archestra/commit/f658cd45f4a9e96a07607b387718cf019635b3f0))


### Dependencies

* **terraform:** bump github.com/hashicorp/terraform-plugin-sdk/v2 from 2.38.1 to 2.38.2 in the terraform-go-dependencies group ([#77](https://github.com/archestra-ai/terraform-provider-archestra/issues/77)) ([cc21eec](https://github.com/archestra-ai/terraform-provider-archestra/commit/cc21eec3cc4141d8de5ca1390ecbbed3a5371c91))

## [0.5.0](https://github.com/archestra-ai/terraform-provider-archestra/compare/v0.4.1...v0.5.0) (2026-01-06)


### Features

* add `archestra_prompt` resource + data sources ([#67](https://github.com/archestra-ai/terraform-provider-archestra/issues/67)) ([8b1f024](https://github.com/archestra-ai/terraform-provider-archestra/commit/8b1f024a286f9bfe22b3633d747c2f33f99188ca))
* add `archestra_sso_provider` resource ([#65](https://github.com/archestra-ai/terraform-provider-archestra/issues/65)) ([37bf7ac](https://github.com/archestra-ai/terraform-provider-archestra/commit/37bf7ac08c90f9dc33bb4be8f17ad3b4bde956e6))

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
