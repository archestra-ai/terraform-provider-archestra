# Examples

Two entry points depending on what you want to do.

## Want to try the provider end-to-end?

→ [demo/](demo/) — a runnable root module that wires the full bring-up
chain together (provider block, org settings, teams, LLM key, MCP
catalog item + install, agents, policies, limits) behind feature flags
for the license-gated paths. Fill in your own values in
`terraform.tfvars` (see `terraform.tfvars.example`), then `terraform
apply` against a real or local backend. The fastest way to see a
realistic configuration without writing a module from scratch.

## Want to copy one resource's shape into your own module?

→ [resources/](resources/) and [data-sources/](data-sources/) — one
illustrative snippet per resource and data source.

**These snippets are not standalone Terraform configurations** — they
don't include a `terraform { required_providers { … } }` block or a
`provider "archestra" {}` block, so running `terraform init` from
inside `examples/resources/<name>/` will fail with "Could not retrieve
the list of available versions for provider hashicorp/archestra".

### Using a snippet

1. Set up your own root module with the provider boilerplate from
   [provider/provider.tf](provider/provider.tf) (`required_providers` +
   `version` pin).
2. Copy the resource block(s) you need from
   [resources/](resources/) or [data-sources/](data-sources/).
3. Most snippets reference *other* resources (e.g.,
   `archestra_team.engineering`) or input variables (e.g.,
   `var.openai_api_key`) — declare those in your own module, or strip
   the cross-references.
4. Apply from your module:

   ```bash
   export ARCHESTRA_BASE_URL="https://archestra.your-company.example"
   export ARCHESTRA_API_KEY="arch_..."
   terraform init
   terraform plan
   ```

The full attribute reference for every resource lives in
[../docs/resources/](../docs/resources/).

## For maintainers

`make generate` (run from the repo root) renders these files into
`../docs/` via `tfplugindocs`. The tool only picks up files at fixed
paths:

- `provider/provider.tf` — provider index page
- `data-sources/<full data source name>/data-source.tf`
- `resources/<full resource name>/resource.tf`

Other `*.tf` files in this tree are ignored by codegen but can still be
applied manually.
