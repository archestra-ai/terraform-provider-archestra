# ====================================================================================
# Setup Project

include ../../../build/makelib/common.mk

# ====================================================================================
#  Options

include ../../../build/makelib/imagelight.mk

# ====================================================================================
# Targets

img.build:
	@$(INFO) docker build $(IMAGE)
	@$(MAKE) BUILD_ARGS="--load" img.build.shared
	@$(OK) docker build $(IMAGE)

img.publish:
	@$(INFO) Skipping image publish for $(IMAGE)
	@echo Publish is deferred to xpkg machinery
	@$(OK) Image publish skipped for $(IMAGE)

img.build.shared:
	@cp Dockerfile $(IMAGE_TEMP_DIR) || $(FAIL)
	@cp terraformrc.hcl $(IMAGE_TEMP_DIR) || $(FAIL)
	@cp -r $(OUTPUT_DIR)/bin/ $(IMAGE_TEMP_DIR)/bin || $(FAIL)
	@# The Dockerfile copies the native provider's zips from provider-zips in
	@# the build context: a release's own verified zips, or else the
	@# published release's, downloaded here.
	@mkdir -p $(IMAGE_TEMP_DIR)/provider-zips || $(FAIL)
	@if [ -n "$(TERRAFORM_PROVIDER_ZIP_DIR)" ]; then \
		cp $(TERRAFORM_PROVIDER_ZIP_DIR)/*.zip $(IMAGE_TEMP_DIR)/provider-zips/ || $(FAIL); \
	else \
		for platform in $(subst $(COMMA), ,$(IMAGE_PLATFORMS)); do \
			zip=$(TERRAFORM_PROVIDER_DOWNLOAD_NAME)_$(TERRAFORM_PROVIDER_VERSION)_$${platform%/*}_$${platform#*/}.zip; \
			curl -fsSL -o $(IMAGE_TEMP_DIR)/provider-zips/$$zip $(TERRAFORM_PROVIDER_DOWNLOAD_URL_PREFIX)/$$zip || exit 1; \
		done || $(FAIL); \
	fi
	@docker buildx build $(BUILD_ARGS) \
		--platform $(IMAGE_PLATFORMS) \
		--build-arg TERRAFORM_VERSION=$(TERRAFORM_VERSION) \
		--build-arg OPENTOFU_SHA256_AMD64=$(OPENTOFU_SHA256_AMD64) \
		--build-arg OPENTOFU_SHA256_ARM64=$(OPENTOFU_SHA256_ARM64) \
		--build-arg TERRAFORM_PROVIDER_SOURCE=$(TERRAFORM_PROVIDER_SOURCE) \
		--build-arg TERRAFORM_PROVIDER_VERSION=$(TERRAFORM_PROVIDER_VERSION) \
		--build-arg TERRAFORM_PROVIDER_DOWNLOAD_NAME=$(TERRAFORM_PROVIDER_DOWNLOAD_NAME) \
		--build-arg TERRAFORM_NATIVE_PROVIDER_BINARY=$(TERRAFORM_NATIVE_PROVIDER_BINARY) \
		-t $(IMAGE) \
		$(IMAGE_TEMP_DIR) || $(FAIL)

img.promote:
	@$(INFO) Skipping image promotion from $(FROM_IMAGE) to $(TO_IMAGE)
	@echo Promote is deferred to xpkg machinery
	@$(OK) Image promotion skipped for $(FROM_IMAGE) to $(TO_IMAGE)
