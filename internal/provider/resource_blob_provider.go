package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ resource.ResourceWithImportState    = &BlobProviderResource{}
	_ resource.ResourceWithValidateConfig = &BlobProviderResource{}
)

func NewBlobProviderResource() resource.Resource { return &BlobProviderResource{} }

// BlobProviderResource manages an organization blob provider: the object
// storage destination that cost exports write to. The API has no single-item
// read, so Read finds the provider in the list. Credentials are write-only: the
// API never returns them, so state keeps what Terraform last sent.
type BlobProviderResource struct{ client *client.ClientWithResponses }

type BlobProviderResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Type               types.String `tfsdk:"type"`
	HasCredentials     types.Bool   `tfsdk:"has_credentials"`
	SkipConnectionTest types.Bool   `tfsdk:"skip_connection_test"`
	Credentials        types.Map    `tfsdk:"credentials"`
	S3                 types.Object `tfsdk:"s3"`
	GCS                types.Object `tfsdk:"gcs"`
	AzureBlob          types.Object `tfsdk:"azure_blob"`
}

// The storage settings live in one wire field, `config`, discriminated by
// `type`. Terraform splits them into one block per provider type, so each
// block is Synthetic here and the resource encodes `config` itself.
var blobProviderAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "s3", JSONName: "config", Kind: Synthetic},
	{TFName: "gcs", JSONName: "config", Kind: Synthetic},
	{TFName: "azure_blob", JSONName: "config", Kind: Synthetic},
	{TFName: "credentials", JSONName: "credentials", Kind: Synthetic, Sensitive: true},
	{TFName: "skip_connection_test", JSONName: "skipConnectionTest", Kind: Synthetic},
}

func (r *BlobProviderResource) AttrSpecs() []AttrSpec { return blobProviderAttrSpec }
func (r *BlobProviderResource) APIShape() any         { return client.ListBlobProvidersResponse{} }

// KnownIntentionallySkipped: audit timestamps and organization ownership are
// server bookkeeping; usedBy lists the cost exports that write here, which
// Terraform already tracks through archestra_cost_export.blob_provider_id.
func (r *BlobProviderResource) KnownIntentionallySkipped() []string {
	return []string{"organizationId", "createdAt", "updatedAt", "usedBy"}
}

func (r *BlobProviderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_blob_provider"
}

func (r *BlobProviderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	exactlyOne := []validator.Object{objectvalidator.ExactlyOneOf(path.MatchRoot("s3"), path.MatchRoot("gcs"), path.MatchRoot("azure_blob"))}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Object storage destination that features such as cost exports write files to (API 1.5.0-rc.30 or later). " +
			"Set exactly one of `s3`, `gcs` or `azure_blob`. By default the API writes and deletes a small test object before saving; set `skip_connection_test` to save without it.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, MarkdownDescription: "Blob provider UUID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":            schema.StringAttribute{Required: true, MarkdownDescription: "Display name.", Validators: []validator.String{stringvalidator.LengthBetween(1, 256), noSurroundingSpace()}},
			"type":            schema.StringAttribute{Computed: true, MarkdownDescription: "Provider type: `s3`, `gcs` or `azure_blob`, from the block you set."},
			"has_credentials": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether static credentials are stored for this provider."},
			"skip_connection_test": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Save without first writing and deleting a test object. Applies to every create and update while set.",
			},
			"credentials": schema.MapAttribute{
				Optional:    true,
				Sensitive:   true,
				ElementType: types.StringType,
				MarkdownDescription: "Write-only static credentials for the auth method: `accessKeyId`, `secretAccessKey` and optional `sessionToken` (S3 `access_key`); " +
					"`serviceAccountKey` (GCS `service_account_key`); `clientSecret`, `accountKey` or `sasToken` (Azure). " +
					"The API never returns them, so Terraform cannot detect out-of-band changes; removing the attribute clears the stored credentials.",
			},
			"s3": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Amazon S3 or an S3-compatible store.",
				Validators:          exactlyOne,
				Attributes: map[string]schema.Attribute{
					"bucket":           schema.StringAttribute{Required: true, MarkdownDescription: "Bucket name.", Validators: []validator.String{stringvalidator.LengthBetween(1, 255), noSurroundingSpace()}},
					"region":           schema.StringAttribute{Required: true, MarkdownDescription: "Bucket region, such as `us-east-1`.", Validators: []validator.String{stringvalidator.LengthBetween(1, 64), noSurroundingSpace()}},
					"key_prefix":       blobKeyPrefixAttribute(),
					"endpoint":         blobEndpointAttribute("Custom endpoint for S3-compatible stores (MinIO, R2, Ceph); omit for AWS."),
					"force_path_style": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Path-style addressing, required by most S3-compatible stores."},
					"auth_method":      schema.StringAttribute{Required: true, MarkdownDescription: "`default_chain` (the platform's AWS credential chain), `access_key` (static `credentials`) or `assume_role`.", Validators: []validator.String{stringvalidator.OneOf("default_chain", "access_key", "assume_role")}},
					"role_arn":         schema.StringAttribute{Optional: true, MarkdownDescription: "IAM role ARN to assume. Required for, and only allowed with, `assume_role`.", Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:[\w-]+:iam::\d{12}:role/.+$`), "Must be an IAM role ARN.")}},
					"external_id":      schema.StringAttribute{Optional: true, MarkdownDescription: "External ID the role's trust policy requires. Only allowed with `assume_role`.", Validators: []validator.String{stringvalidator.LengthBetween(1, 1224), noSurroundingSpace()}},
				},
			},
			"gcs": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Google Cloud Storage.",
				Validators:          exactlyOne,
				Attributes: map[string]schema.Attribute{
					"bucket":      schema.StringAttribute{Required: true, MarkdownDescription: "Bucket name.", Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,220}[a-z0-9]$`), "Must be a Cloud Storage bucket name.")}},
					"key_prefix":  blobKeyPrefixAttribute(),
					"auth_method": schema.StringAttribute{Required: true, MarkdownDescription: "`application_default` (the platform's credentials) or `service_account_key` (static `credentials`).", Validators: []validator.String{stringvalidator.OneOf("application_default", "service_account_key")}},
				},
			},
			"azure_blob": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Azure Blob Storage.",
				Validators:          exactlyOne,
				Attributes: map[string]schema.Attribute{
					"account_name": schema.StringAttribute{Required: true, MarkdownDescription: "Storage account name.", Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[a-z0-9]{3,24}$`), "Must be 3-24 lowercase letters and digits.")}},
					"container":    schema.StringAttribute{Required: true, MarkdownDescription: "Container name.", Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`), "Must be a container name (3-63 lowercase letters, digits and single dashes)."), stringvalidator.RegexMatches(regexp.MustCompile(`^(?:[^-]|-[^-])*-?$`), "Must not contain consecutive dashes.")}},
					"key_prefix":   blobKeyPrefixAttribute(),
					"endpoint":     blobEndpointAttribute("Blob service endpoint for sovereign clouds or Azurite; omit for `https://<account_name>.blob.core.windows.net`."),
					"auth_method":  schema.StringAttribute{Required: true, MarkdownDescription: "`default_credential`, `service_principal`, `account_key` or `sas_token`.", Validators: []validator.String{stringvalidator.OneOf(azureBlobAuthMethods...)}},
					"tenant_id":    schema.StringAttribute{Optional: true, MarkdownDescription: "Entra tenant ID. Required for, and only allowed with, `service_principal`.", Validators: []validator.String{stringvalidator.LengthBetween(1, 64), noSurroundingSpace()}},
					"client_id":    schema.StringAttribute{Optional: true, MarkdownDescription: "App registration client ID. Required for, and only allowed with, `service_principal`.", Validators: []validator.String{stringvalidator.LengthBetween(1, 64), noSurroundingSpace()}},
				},
			},
		},
	}
}

func (r *BlobProviderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	api, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.ClientWithResponses, got %T", req.ProviderData))
		return
	}
	r.client = api
}

// ValidateConfig rejects auth-specific fields the API would silently drop, so
// the saved provider always matches the configuration.
func (r *BlobProviderResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data BlobProviderResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if s3 := blobBlockValues(data.S3); s3 != nil {
		requireOnlyWith(resp, "s3", s3, "auth_method", "assume_role", []string{"role_arn"}, []string{"external_id"})
	}
	if azure := blobBlockValues(data.AzureBlob); azure != nil {
		requireOnlyWith(resp, "azure_blob", azure, "auth_method", "service_principal", []string{"tenant_id", "client_id"}, nil)
	}
}

func (r *BlobProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BlobProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{"name": plan.Name.ValueString(), "config": blobProviderConfig(plan, &resp.Diagnostics)}
	if creds := blobCredentials(ctx, plan.Credentials, &resp.Diagnostics); creds != nil {
		body["credentials"] = creds
	}
	if plan.SkipConnectionTest.ValueBool() {
		body["skipConnectionTest"] = true
	}
	if resp.Diagnostics.HasError() {
		return
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.CreateBlobProviderWithBodyWithResponse(ctx, "application/json", bytes.NewReader(encoded))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Blob Provider Creation Failed", apiFailure(result.StatusCode(), result.Body))
		return
	}
	plan.ID = types.StringValue(result.JSON200.Id.String())
	flattenBlobProvider(&plan, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BlobProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BlobProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if state.ID.IsNull() || state.ID.IsUnknown() || state.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	raw, found := r.find(ctx, state.ID.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	flattenBlobProvider(&state, raw, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BlobProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior BlobProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{}
	if !plan.Name.Equal(prior.Name) {
		body["name"] = plan.Name.ValueString()
	}
	if !plan.S3.Equal(prior.S3) || !plan.GCS.Equal(prior.GCS) || !plan.AzureBlob.Equal(prior.AzureBlob) {
		body["config"] = blobProviderConfig(plan, &resp.Diagnostics)
	}
	if !plan.Credentials.Equal(prior.Credentials) {
		if creds := blobCredentials(ctx, plan.Credentials, &resp.Diagnostics); creds != nil {
			body["credentials"] = creds
		} else {
			body["clearCredentials"] = true
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	if len(body) == 0 {
		// Only skip_connection_test changed; it has no stored value.
		plan.Type, plan.HasCredentials = prior.Type, prior.HasCredentials
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}
	if plan.SkipConnectionTest.ValueBool() {
		body["skipConnectionTest"] = true
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	LogPatch(ctx, "archestra_blob_provider Update", body, blobProviderAttrSpec)
	id, ok := parseUUID(prior.ID.ValueString(), &resp.Diagnostics)
	if !ok {
		return
	}
	result, err := r.client.UpdateBlobProviderWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(encoded))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Blob Provider Update Failed", apiFailure(result.StatusCode(), result.Body))
		return
	}
	plan.ID = prior.ID
	flattenBlobProvider(&plan, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BlobProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BlobProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, ok := parseUUID(state.ID.ValueString(), &resp.Diagnostics)
	if !ok {
		return
	}
	result, err := r.client.DeleteBlobProviderWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if IsNotFound(result) {
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Blob Provider Deletion Failed", apiFailure(result.StatusCode(), result.Body))
	}
}

func (r *BlobProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// find returns the provider's JSON from the list endpoint, the only read the
// API offers.
func (r *BlobProviderResource) find(ctx context.Context, id string, diags *diag.Diagnostics) (json.RawMessage, bool) {
	return findBlobProvider(ctx, r.client, func(p blobProviderWire) bool { return p.ID == id }, diags)
}

// =============================================================================
// Shared with the data source and tests
// =============================================================================

type blobProviderWire struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Type           string          `json:"type"`
	HasCredentials bool            `json:"hasCredentials"`
	Config         json.RawMessage `json:"config"`
}

// findBlobProvider lists providers and returns the raw JSON of the single one
// matching. More than one match is an error.
func findBlobProvider(ctx context.Context, api *client.ClientWithResponses, match func(blobProviderWire) bool, diags *diag.Diagnostics) (json.RawMessage, bool) {
	result, err := api.ListBlobProvidersWithResponse(ctx)
	if err != nil {
		diags.AddError("API Error", err.Error())
		return nil, false
	}
	if result.JSON200 == nil {
		diags.AddError("Blob Provider Read Failed", apiFailure(result.StatusCode(), result.Body))
		return nil, false
	}
	var items []json.RawMessage
	if err := json.Unmarshal(result.Body, &items); err != nil {
		diags.AddError("Invalid Blob Provider Response", err.Error())
		return nil, false
	}
	var found json.RawMessage
	for _, item := range items {
		var wire blobProviderWire
		if err := json.Unmarshal(item, &wire); err != nil {
			diags.AddError("Invalid Blob Provider Response", err.Error())
			return nil, false
		}
		if !match(wire) {
			continue
		}
		if found != nil {
			diags.AddError("Ambiguous Blob Provider", fmt.Sprintf("More than one blob provider is named %q; reference it by ID.", wire.Name))
			return nil, false
		}
		found = item
	}
	return found, found != nil
}

var (
	s3BlockType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"bucket": types.StringType, "region": types.StringType, "key_prefix": types.StringType, "endpoint": types.StringType,
		"force_path_style": types.BoolType, "auth_method": types.StringType, "role_arn": types.StringType, "external_id": types.StringType,
	}}
	gcsBlockType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"bucket": types.StringType, "key_prefix": types.StringType, "auth_method": types.StringType,
	}}
	azureBlobBlockType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"account_name": types.StringType, "container": types.StringType, "key_prefix": types.StringType, "endpoint": types.StringType,
		"auth_method": types.StringType, "tenant_id": types.StringType, "client_id": types.StringType,
	}}
	azureBlobAuthMethods = []string{"default_credential", "service_principal", "account_key", "sas_token"}
)

// blobBlockFields maps each block's Terraform attributes to config keys.
var blobBlockFields = map[string][][2]string{
	"s3": {{"bucket", "bucket"}, {"region", "region"}, {"key_prefix", "keyPrefix"}, {"endpoint", "endpoint"},
		{"force_path_style", "forcePathStyle"}, {"auth_method", "authMethod"}, {"role_arn", "roleArn"}, {"external_id", "externalId"}},
	"gcs": {{"bucket", "bucket"}, {"key_prefix", "keyPrefix"}, {"auth_method", "authMethod"}},
	"azure_blob": {{"account_name", "accountName"}, {"container", "container"}, {"key_prefix", "keyPrefix"}, {"endpoint", "endpoint"},
		{"auth_method", "authMethod"}, {"tenant_id", "tenantId"}, {"client_id", "clientId"}},
}

// blobProviderConfig encodes the set block as the API's `config` union.
func blobProviderConfig(m BlobProviderResourceModel, diags *diag.Diagnostics) map[string]any {
	for _, block := range []struct {
		kind string
		obj  types.Object
	}{{"s3", m.S3}, {"gcs", m.GCS}, {"azure_blob", m.AzureBlob}} {
		values := blobBlockValues(block.obj)
		if values == nil {
			continue
		}
		config := map[string]any{"type": block.kind}
		for _, field := range blobBlockFields[block.kind] {
			switch v := values[field[0]].(type) {
			case basetypes.StringValue:
				if !v.IsNull() && !v.IsUnknown() {
					config[field[1]] = v.ValueString()
				}
			case basetypes.BoolValue:
				if !v.IsNull() && !v.IsUnknown() {
					config[field[1]] = v.ValueBool()
				}
			}
		}
		return config
	}
	diags.AddError("Missing Storage Settings", "Set exactly one of s3, gcs or azure_blob.")
	return nil
}

// flattenBlobProvider sets the computed fields and the block for the stored
// type from a provider's JSON. Credentials and skip_connection_test are not
// returned, so they keep their current values.
func flattenBlobProvider(m *BlobProviderResourceModel, raw []byte, diags *diag.Diagnostics) {
	var wire blobProviderWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		diags.AddError("Invalid Blob Provider Response", err.Error())
		return
	}
	var config map[string]any
	if err := json.Unmarshal(wire.Config, &config); err != nil {
		diags.AddError("Invalid Blob Provider Response", "Unable to decode config: "+err.Error())
		return
	}
	m.ID = types.StringValue(wire.ID)
	m.Name = types.StringValue(wire.Name)
	m.Type = types.StringValue(wire.Type)
	m.HasCredentials = types.BoolValue(wire.HasCredentials)
	m.S3, m.GCS, m.AzureBlob = types.ObjectNull(s3BlockType.AttrTypes), types.ObjectNull(gcsBlockType.AttrTypes), types.ObjectNull(azureBlobBlockType.AttrTypes)
	blockType := map[string]types.ObjectType{"s3": s3BlockType, "gcs": gcsBlockType, "azure_blob": azureBlobBlockType}[wire.Type]
	if blockType.AttrTypes == nil {
		diags.AddError("Unsupported Blob Provider Type", fmt.Sprintf("This provider version does not support blob provider type %q; upgrade the provider.", wire.Type))
		return
	}
	values := map[string]attr.Value{}
	for _, field := range blobBlockFields[wire.Type] {
		if blockType.AttrTypes[field[0]] == types.BoolType {
			b, _ := config[field[1]].(bool)
			values[field[0]] = types.BoolValue(b)
			continue
		}
		if s, ok := config[field[1]].(string); ok {
			values[field[0]] = types.StringValue(s)
		} else {
			values[field[0]] = types.StringNull()
		}
	}
	obj, d := types.ObjectValue(blockType.AttrTypes, values)
	diags.Append(d...)
	switch wire.Type {
	case "s3":
		m.S3 = obj
	case "gcs":
		m.GCS = obj
	case "azure_blob":
		m.AzureBlob = obj
	}
}

func blobBlockValues(obj types.Object) map[string]attr.Value {
	if obj.IsNull() || obj.IsUnknown() {
		return nil
	}
	return obj.Attributes()
}

func blobCredentials(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]string {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	creds := map[string]string{}
	diags.Append(m.ElementsAs(ctx, &creds, false)...)
	if len(creds) == 0 {
		return nil
	}
	return creds
}

// requireOnlyWith checks that `fields` are set when attr equals value and
// `optional` fields appear only then.
func requireOnlyWith(resp *resource.ValidateConfigResponse, block string, values map[string]attr.Value, attrName, value string, required, optional []string) {
	method, ok := values[attrName].(basetypes.StringValue)
	if !ok || method.IsUnknown() || method.IsNull() {
		return
	}
	isValue := method.ValueString() == value
	for _, name := range append(append([]string{}, required...), optional...) {
		v, _ := values[name].(basetypes.StringValue)
		if v.IsUnknown() {
			continue
		}
		if !isValue && !v.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root(block).AtName(name), "Unused Setting", fmt.Sprintf("%s is only used with %s = %q; remove it.", name, attrName, value))
		}
	}
	if !isValue {
		return
	}
	for _, name := range required {
		v, _ := values[name].(basetypes.StringValue)
		if !v.IsUnknown() && v.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root(block).AtName(name), "Missing Setting", fmt.Sprintf("%s = %q needs %s.", attrName, value, name))
		}
	}
}

// blobKeyPrefixAttribute rejects surrounding slashes, which the API strips, so
// the stored prefix always equals the configured one.
func blobKeyPrefixAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true, Computed: true, Default: stringdefault.StaticString(""),
		MarkdownDescription: "Key prefix inside the bucket that every object is written under, without leading or trailing `/`.",
		Validators: []validator.String{
			stringvalidator.LengthAtMost(512), noSurroundingSpace(),
			stringvalidator.RegexMatches(regexp.MustCompile(`^(?:[^/].*[^/]|[^/])?$`), "Must not start or end with /."),
		},
	}
}

func blobEndpointAttribute(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true, MarkdownDescription: description,
		Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^https?://[^\s]*[^/\s]$`), "Must be an http(s) URL without a trailing /.")},
	}
}

// apiFailure describes a failed API call, using the API's error message when
// the body carries one.
func apiFailure(status int, body []byte) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Error.Message != "" {
		return fmt.Sprintf("API returned status %d: %s", status, envelope.Error.Message)
	}
	return fmt.Sprintf("API returned status %d", status)
}

func parseUUID(value string, diags *diag.Diagnostics) (uuid.UUID, bool) {
	id, err := uuid.Parse(value)
	if err != nil {
		diags.AddError("Invalid ID", fmt.Sprintf("%q is not a UUID: %s", value, err))
		return uuid.UUID{}, false
	}
	return id, true
}

func noSurroundingSpace() validator.String {
	return stringvalidator.RegexMatches(regexp.MustCompile(`^(?:\S(?:.*\S)?)?$`), "Must not start or end with whitespace.")
}
