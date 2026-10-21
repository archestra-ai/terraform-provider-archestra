package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func NewKnowledgeFileResource() resource.Resource { return &KnowledgeFileResource{} }

type KnowledgeFileResource struct{ client *client.ClientWithResponses }
type KnowledgeFileResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Filename         types.String `tfsdk:"filename"`
	MIMEType         types.String `tfsdk:"mime_type"`
	ContentBase64    types.String `tfsdk:"content_base64"`
	DirectoryID      types.String `tfsdk:"directory_id"`
	Labels           types.Set    `tfsdk:"labels"`
	Permissions      types.Set    `tfsdk:"permissions"`
	SHA256           types.String `tfsdk:"sha256"`
	SizeBytes        types.Int64  `tfsdk:"size_bytes"`
	KnowledgeBaseIDs types.Set    `tfsdk:"knowledge_base_ids"`
	CreatedAt        types.String `tfsdk:"created_at"`
}

var knowledgeFileLabelType = types.ObjectType{AttrTypes: map[string]attr.Type{"key": types.StringType, "value": types.StringType}}
var knowledgeFileAttrSpec = []AttrSpec{
	{TFName: "sha256", ReadJSONName: "contentHash", Kind: Synthetic},
	{TFName: "knowledge_base_ids", ReadJSONName: "knowledgeBases", Kind: Synthetic},
	{TFName: "filename", JSONName: "filename", Kind: Scalar},
	{TFName: "mime_type", JSONName: "mimeType", Kind: Scalar, CreateOnly: true},
	{TFName: "content_base64", JSONName: "content", Kind: Scalar, Sensitive: true, CreateOnly: true},
	{TFName: "directory_id", JSONName: "directoryId", Kind: Scalar},
	{TFName: "labels", JSONName: "labels", Kind: Set, OmitOnNull: true, Children: []AttrSpec{{TFName: "key", JSONName: "key", Kind: Scalar}, {TFName: "value", JSONName: "value", Kind: Scalar}}},
	initialGrantsAttrSpec,
}

func (r *KnowledgeFileResource) AttrSpecs() []AttrSpec { return knowledgeFileAttrSpec }
func (r *KnowledgeFileResource) APIShape() any         { return client.UploadKnowledgeFileResponse{} }
func (r *KnowledgeFileResource) KnownIntentionallySkipped() []string {
	return []string{"organizationId", "visibility", "createdBy", "createdByServiceAccountId", "teamIds"}
}
func (r *KnowledgeFileResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_knowledge_file"
}
func (r *KnowledgeFileResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "An uploaded document in Knowledge. Owns its bytes, filename, directory, labels and sharing. Upload does not run indexing; knowledge_base_ids reports existing indexed associations. Content or MIME-type changes replace the file because the standalone upload API has no content-update operation. Destroy also removes documents indexed from this file. Imported bytes are read into sensitive Terraform state.", Attributes: map[string]schema.Attribute{
		"id":                 schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Knowledge file UUID."},
		"filename":           schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 512), knowledgeFileTrimmedValidator{}}, MarkdownDescription: "Document filename, including an extension the Knowledge importer supports. Can be renamed in place."},
		"mime_type":          schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 255), knowledgeFileTrimmedValidator{}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Stored MIME type. Changes replace the file."},
		"content_base64":     schema.StringAttribute{Required: true, Sensitive: true, Validators: []validator.String{knowledgeFileContentValidator{}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Canonical base64 document bytes; use filebase64 or base64encode. Must contain at least one byte. Changes replace the file; the API enforces the configured upload-size and supported-format limits."},
		"directory_id":       schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical directory UUID.")}, MarkdownDescription: "Knowledge directory UUID. Omit to place the file at the repository root; removing this field moves it back to root."},
		"labels":             schema.SetNestedAttribute{Optional: true, Computed: true, MarkdownDescription: "Key/value labels. Omit to preserve labels; [] clears them.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{"key": schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}}, "value": schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}}}}},
		"permissions":        initialGrantsAttribute(),
		"sha256":             schema.StringAttribute{Computed: true, MarkdownDescription: "SHA-256 verified against the actual stored document bytes."},
		"size_bytes":         schema.Int64Attribute{Computed: true, MarkdownDescription: "Observed byte length."},
		"knowledge_base_ids": schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Knowledge bases this file is currently indexed into. Indexing is observed here; uploading alone does not index a file."},
		"created_at":         schema.StringAttribute{Computed: true, MarkdownDescription: "Creation timestamp."},
	}}
}
func (r *KnowledgeFileResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected API client, got %T", req.ProviderData))
		return
	}
	r.client = c
}
func (r *KnowledgeFileResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
func (r *KnowledgeFileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer finishParentCreate(ctx, r.client, "knowledgeFile", "id", resp)
	var data KnowledgeFileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), knowledgeFileAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.UploadKnowledgeFileWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Knowledge file upload failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	if !r.flatten(ctx, &data, result.Body, &resp.Diagnostics) {
		return
	}
	resolveInitialGrants(&data.Permissions)
	// The upload succeeded. Retain the identity before any byte/policy follow-up.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.refresh(ctx, &data, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Knowledge file not found", "The uploaded file was not returned by the repository. Its identity remains in state for normal cleanup.")
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *KnowledgeFileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data KnowledgeFileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	if !r.refresh(ctx, &data, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	readParentPermissions(ctx, r.client, "knowledgeFile", data.ID.ValueString(), &data.Permissions, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *KnowledgeFileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	finish := beginParentPermissionUpdate(ctx, r.client, "knowledgeFile", "id", req, resp)
	defer finish()
	if resp.Diagnostics.HasError() {
		return
	}
	var data KnowledgeFileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid file UUID", err.Error())
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, knowledgeFileAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) > 0 {
		body, err := json.Marshal(patch)
		if err != nil {
			resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
			return
		}
		result, err := r.client.UpdateKnowledgeFileWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
		if err != nil {
			resp.Diagnostics.AddError("API Error", err.Error())
			return
		}
		if result.JSON200 == nil {
			resp.Diagnostics.AddError("Knowledge file update failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
			return
		}
	}
	if !r.refresh(ctx, &data, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Knowledge file not found", "Refresh and replan before updating a missing file.")
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *KnowledgeFileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data KnowledgeFileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid file UUID", err.Error())
		return
	}
	result, err := r.client.DeleteKnowledgeFileWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
		resp.Diagnostics.AddError("Knowledge file deletion failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}
func (r *KnowledgeFileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := uuid.Parse(req.ID)
	if err != nil || id.String() != req.ID {
		resp.Diagnostics.AddError("Invalid file UUID", "Import a lowercase canonical Knowledge file UUID.")
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func (r *KnowledgeFileResource) refresh(ctx context.Context, data *KnowledgeFileResourceModel, diags *diag.Diagnostics) bool {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid file UUID", err.Error())
		return false
	}
	limit, offset := 100, 0
	for {
		result, err := r.client.GetKnowledgeFilesWithResponse(ctx, &client.GetKnowledgeFilesParams{Limit: &limit, Offset: &offset})
		if err != nil {
			diags.AddError("API Error", err.Error())
			return false
		}
		if result.JSON200 == nil {
			diags.AddError("Knowledge file listing failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
			return false
		}
		for _, row := range result.JSON200.Data {
			if row.Id != id {
				continue
			}
			body, err := json.Marshal(row)
			if err != nil {
				diags.AddError("Response Encoding Failed", err.Error())
				return false
			}
			if !r.flatten(ctx, data, body, diags) {
				return false
			}
			content, err := r.client.GetKnowledgeFileContentWithResponse(ctx, id)
			if err != nil {
				diags.AddError("Knowledge file content read failed", err.Error())
				return false
			}
			if content.StatusCode() != 200 {
				diags.AddError("Knowledge file content read failed", fmt.Sprintf("API returned status %d; the resource identity is retained.", content.StatusCode()))
				return false
			}
			digest := fmt.Sprintf("%x", sha256.Sum256(content.Body))
			if digest != data.SHA256.ValueString() || int64(len(content.Body)) != data.SizeBytes.ValueInt64() {
				diags.AddError("Knowledge file changed during read", "The listed hash/size does not match the downloaded bytes. Refresh and replan before changing the document.")
				return false
			}
			data.ContentBase64 = types.StringValue(base64.StdEncoding.EncodeToString(content.Body))
			return true
		}
		if !result.JSON200.Pagination.HasNext {
			// Offset pagination can miss a row while other documents move. The
			// direct byte endpoint must confirm absence before dropping identity.
			content, err := r.client.GetKnowledgeFileContentWithResponse(ctx, id)
			if err != nil {
				diags.AddError("Knowledge file existence check failed", err.Error())
			} else if !IsNotFound(content) {
				diags.AddError("Knowledge file metadata unavailable", fmt.Sprintf("The document was absent from the repository listing, but its direct read returned status %d. Refresh and replan; its identity is retained.", content.StatusCode()))
			}
			return false
		}
		if len(result.JSON200.Data) == 0 {
			diags.AddError("Invalid repository pagination", "The API reported another page without returning any rows.")
			return false
		}
		offset += len(result.JSON200.Data)
	}
}
func (r *KnowledgeFileResource) flatten(ctx context.Context, data *KnowledgeFileResourceModel, body []byte, diags *diag.Diagnostics) bool {
	var row struct {
		ID          string     `json:"id"`
		Filename    string     `json:"filename"`
		MIMEType    string     `json:"mimeType"`
		DirectoryID *string    `json:"directoryId"`
		ContentHash string     `json:"contentHash"`
		SizeBytes   int64      `json:"sizeBytes"`
		CreatedAt   *time.Time `json:"createdAt"`
		Labels      []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"labels"`
		KnowledgeBases []struct {
			ID string `json:"id"`
		} `json:"knowledgeBases"`
	}
	if err := json.Unmarshal(body, &row); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return false
	}
	if row.ID == "" {
		diags.AddError("Invalid API Response", "The document has no identity.")
		return false
	}
	if !data.ID.IsNull() && !data.ID.IsUnknown() && data.ID.ValueString() != row.ID {
		diags.AddError("Unexpected document identity", "The response belongs to a different Knowledge file.")
		return false
	}
	data.ID = types.StringValue(row.ID)
	data.Filename = types.StringValue(row.Filename)
	data.MIMEType = types.StringValue(row.MIMEType)
	data.DirectoryID = types.StringPointerValue(row.DirectoryID)
	data.SHA256 = types.StringValue(row.ContentHash)
	data.SizeBytes = types.Int64Value(row.SizeBytes)
	data.CreatedAt = canonicalAPITimestamp(row.CreatedAt)
	labels := make([]attr.Value, 0, len(row.Labels))
	for _, l := range row.Labels {
		labels = append(labels, types.ObjectValueMust(knowledgeFileLabelType.AttrTypes, map[string]attr.Value{"key": types.StringValue(l.Key), "value": types.StringValue(l.Value)}))
	}
	var d diag.Diagnostics
	data.Labels, d = types.SetValue(knowledgeFileLabelType, labels)
	diags.Append(d...)
	ids := make([]string, 0, len(row.KnowledgeBases))
	for _, kb := range row.KnowledgeBases {
		ids = append(ids, kb.ID)
	}
	data.KnowledgeBaseIDs, d = types.SetValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)
	return !diags.HasError()
}

type knowledgeFileTrimmedValidator struct{}

func (knowledgeFileTrimmedValidator) Description(context.Context) string {
	return "Use a value without surrounding whitespace."
}
func (v knowledgeFileTrimmedValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (knowledgeFileTrimmedValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	v := req.ConfigValue.ValueString()
	if strings.TrimSpace(v) != v {
		resp.Diagnostics.AddAttributeError(req.Path, "Surrounding whitespace is not supported", "The API trims this field. Remove surrounding whitespace to keep configuration and stored state identical.")
	}
}

type knowledgeFileContentValidator struct{}

func (knowledgeFileContentValidator) Description(context.Context) string {
	return "Use canonical base64 encoding of a nonempty document."
}
func (v knowledgeFileContentValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (knowledgeFileContentValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	v := req.ConfigValue.ValueString()
	raw, err := base64.StdEncoding.Strict().DecodeString(v)
	if err != nil || len(raw) == 0 || base64.StdEncoding.EncodeToString(raw) != v {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid document bytes", "Use canonical base64 encoding of at least one byte. The API validates supported formats and its configured upload-size limit.")
	}
}
