# OpenTofu (MPL-2.0), installed as `terraform`, the binary upjet runs. It is
# downloaded and checked against the release's SHA-256 (set with the version
# in crossplane/Makefile) in a stage of its own, so the image copies a
# verified binary and adds nothing from a URL.
FROM dhi.io/alpine-base:3.23-dev@sha256:c8b128dc993e8cdaf2972ea5b005b74363c634ba499ae6a99a41f22504899472 AS runtime-base

FROM runtime-base AS opentofu
ARG TARGETOS
ARG TARGETARCH
ARG TERRAFORM_VERSION
ARG OPENTOFU_SHA256_AMD64
ARG OPENTOFU_SHA256_ARM64
RUN case "${TARGETARCH}" in \
      amd64) sum="${OPENTOFU_SHA256_AMD64}" ;; \
      arm64) sum="${OPENTOFU_SHA256_ARM64}" ;; \
      *) echo "unsupported architecture ${TARGETARCH}" >&2; exit 1 ;; \
    esac \
 && [ -n "${sum}" ] \
 && wget -q -O /tmp/tofu.zip "https://github.com/opentofu/opentofu/releases/download/v${TERRAFORM_VERSION}/tofu_${TERRAFORM_VERSION}_${TARGETOS}_${TARGETARCH}.zip" \
 && echo "${sum}  /tmp/tofu.zip" | sha256sum -c - \
 && unzip /tmp/tofu.zip tofu -d /out \
 && chmod 0755 /out/tofu

FROM runtime-base
RUN apk upgrade --no-cache && apk --no-cache add ca-certificates bash

ARG TARGETOS
ARG TARGETARCH

COPY "bin/${TARGETOS}_${TARGETARCH}/provider" /usr/local/bin/provider

ENV USER_ID=65532

# Setup Terraform environment

## Provider-dependent configuration
ARG TERRAFORM_VERSION
ARG TERRAFORM_PROVIDER_SOURCE
ARG TERRAFORM_PROVIDER_VERSION
ARG TERRAFORM_PROVIDER_DOWNLOAD_NAME
ARG TERRAFORM_NATIVE_PROVIDER_BINARY

## End of - Provider-dependent configuration

ENV PLUGIN_DIR=/terraform/provider-mirror/registry.terraform.io/${TERRAFORM_PROVIDER_SOURCE}/${TERRAFORM_PROVIDER_VERSION}/${TARGETOS}_${TARGETARCH}
ENV TF_CLI_CONFIG_FILE=/terraform/.terraformrc
ENV TF_FORK=0

RUN mkdir -p ${PLUGIN_DIR}

COPY --from=opentofu /out/tofu /usr/local/bin/terraform
# The provider's release zip, which the image Makefile puts in provider-zips.
COPY provider-zips/${TERRAFORM_PROVIDER_DOWNLOAD_NAME}_${TERRAFORM_PROVIDER_VERSION}_${TARGETOS}_${TARGETARCH}.zip /tmp/
COPY terraformrc.hcl ${TF_CLI_CONFIG_FILE}

RUN unzip /tmp/${TERRAFORM_PROVIDER_DOWNLOAD_NAME}_${TERRAFORM_PROVIDER_VERSION}_${TARGETOS}_${TARGETARCH}.zip -d ${PLUGIN_DIR} \
  && chmod +x ${PLUGIN_DIR}/* \
  && rm /tmp/${TERRAFORM_PROVIDER_DOWNLOAD_NAME}_${TERRAFORM_PROVIDER_VERSION}_${TARGETOS}_${TARGETARCH}.zip \
  && chown -R ${USER_ID}:${USER_ID} /terraform
# End of - Setup Terraform environment

# Provider controller needs these environment variable at runtime
ENV TERRAFORM_VERSION=${TERRAFORM_VERSION}
ENV TERRAFORM_PROVIDER_SOURCE=${TERRAFORM_PROVIDER_SOURCE}
ENV TERRAFORM_PROVIDER_VERSION=${TERRAFORM_PROVIDER_VERSION}
ENV TERRAFORM_NATIVE_PROVIDER_PATH=${PLUGIN_DIR}/${TERRAFORM_NATIVE_PROVIDER_BINARY}

USER ${USER_ID}
EXPOSE 8080

ENTRYPOINT ["provider"]
