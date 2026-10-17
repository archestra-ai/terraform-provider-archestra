package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func NewProjectFileResource() resource.Resource { return &ProjectFileResource{} }

type ProjectFileResource struct{ client *client.ClientWithResponses }
type ProjectFileResourceModel struct {
	ID             types.String `tfsdk:"id"`
	ProjectID      types.String `tfsdk:"project_id"`
	FileID         types.String `tfsdk:"file_id"`
	Filename       types.String `tfsdk:"filename"`
	ContentBase64  types.String `tfsdk:"content_base64"`
	UploadMIMEType types.String `tfsdk:"upload_mime_type"`
	MIMEType       types.String `tfsdk:"mime_type"`
	SHA256         types.String `tfsdk:"sha256"`
	SizeBytes      types.Int64  `tfsdk:"size_bytes"`
	DownloadRef    types.String `tfsdk:"download_ref"`
	Downloadable   types.Bool   `tfsdk:"downloadable"`
	ProjectName    types.String `tfsdk:"project_name"`
	CreatedAt      types.String `tfsdk:"created_at"`
}

var projectFileAttrSpec = []AttrSpec{
	{TFName: "project_id", Kind: Synthetic},
	{TFName: "filename", JSONName: "name", Kind: Scalar},
	{TFName: "content_base64", JSONName: "dataBase64", Kind: Scalar, Sensitive: true},
	{TFName: "upload_mime_type", JSONName: "mimeType", Kind: Scalar},
}

func (r *ProjectFileResource) AttrSpecs() []AttrSpec               { return projectFileAttrSpec }
func (r *ProjectFileResource) APIShape() any                       { return client.GetProjectFilesResponse{} }
func (r *ProjectFileResource) KnownIntentionallySkipped() []string { return nil }
func (r *ProjectFileResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_file"
}
func (r *ProjectFileResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Project-owned file bytes with scoped import and deletion. UTF-8 Markdown/text content up to 1,000,000 bytes updates in place; other content changes replace the file. Initial uploads require at least one byte. Filename collisions fail without overwriting the existing file; project instructions use the project's instructions attribute.", Attributes: map[string]schema.Attribute{
		"id":               schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Composite identity project_uuid:file_uuid."},
		"project_id":       schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical project UUID.")}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Owning project UUID."},
		"file_id":          schema.StringAttribute{Computed: true, MarkdownDescription: "Observed file-row UUID."},
		"filename":         schema.StringAttribute{Required: true, Validators: []validator.String{projectFilenameValidator{}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Exact safe basename, excluding the reserved instructions.md file."},
		"content_base64":   schema.StringAttribute{Required: true, Sensitive: true, Validators: []validator.String{projectFileBase64Validator{}}, MarkdownDescription: "Canonical base64 bytes; use filebase64 or base64encode in configuration."},
		"upload_mime_type": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Creation-only MIME hint; omission uses the API default."},
		"mime_type":        schema.StringAttribute{Computed: true, MarkdownDescription: "Actual stored MIME type, independent of the download response's safe Content-Type."},
		"sha256":           schema.StringAttribute{Computed: true, MarkdownDescription: "SHA-256 of the observed bytes."},
		"size_bytes":       schema.Int64Attribute{Computed: true, MarkdownDescription: "Observed byte length."},
		"download_ref":     schema.StringAttribute{Computed: true, MarkdownDescription: "Stable artifact download handle."},
		"downloadable":     schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the backing file is available for download."},
		"project_name":     schema.StringAttribute{Computed: true, MarkdownDescription: "Observed project name."},
		"created_at":       schema.StringAttribute{Computed: true, MarkdownDescription: "File creation timestamp."},
	}}
}
func (r *ProjectFileResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	value, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected API client, got %T", req.ProviderData))
		return
	}
	r.client = value
}
func (r *ProjectFileResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan, prior ProjectFileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !req.State.Raw.IsNull() {
		resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	if plan.ContentBase64.IsUnknown() {
		if !req.State.Raw.IsNull() {
			resp.RequiresReplace.Append(path.Root("content_base64"))
		}
		return
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(plan.ContentBase64.ValueString())
	if err != nil {
		return
	}
	replacing := req.State.Raw.IsNull() || !plan.Filename.Equal(prior.Filename) || !plan.ProjectID.Equal(prior.ProjectID) || !plan.UploadMIMEType.Equal(prior.UploadMIMEType)
	if len(decoded) == 0 && (replacing || !projectFileEditable(plan.Filename.ValueString(), decoded)) {
		resp.Diagnostics.AddAttributeError(path.Root("content_base64"), "Empty initial upload", "The project upload API requires at least one byte. Existing editable text files can be cleared in place.")
		return
	}
	if !req.State.Raw.IsNull() && !plan.ContentBase64.Equal(prior.ContentBase64) && !projectFileEditable(plan.Filename.ValueString(), decoded) {
		resp.RequiresReplace.Append(path.Root("content_base64"))
	}
}
func (r *ProjectFileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProjectFileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectID, err := uuid.Parse(data.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid project UUID", err.Error())
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), projectFileAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "Project file Create", patch, projectFileAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.UploadProjectFilesWithBodyWithResponse(ctx, projectID, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Upload Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	fileID := result.JSON200.Id.String()
	data.ID = types.StringValue(projectID.String() + ":" + fileID)
	data.FileID = types.StringValue(fileID)
	data.MIMEType = types.StringValue(result.JSON200.MimeType)
	data.DownloadRef = types.StringValue(fileID)
	data.Downloadable = types.BoolValue(true)
	data.ProjectName = types.StringNull()
	data.CreatedAt = types.StringNull()
	raw, err := base64.StdEncoding.Strict().DecodeString(data.ContentBase64.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid file bytes", err.Error())
		return
	}
	setProjectFileBytes(&data, raw)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if result.JSON200.Filename != data.Filename.ValueString() {
		cleanup, err := r.client.DeleteSkillSandboxArtifactWithResponse(ctx, fileID)
		if err == nil && (cleanup.StatusCode() == 200 || cleanup.StatusCode() == 204 || IsNotFound(cleanup)) {
			resp.State.RemoveResource(ctx)
		}
		resp.Diagnostics.AddError("Project file name collision", "The API renamed the new upload because the requested filename exists. The existing file was preserved; import its scoped identity or choose another filename.")
		if err != nil {
			resp.Diagnostics.AddError("Collision cleanup failed", "The created identity remains in state for normal cleanup.")
		} else if cleanup.StatusCode() != 200 && cleanup.StatusCode() != 204 && !IsNotFound(cleanup) {
			resp.Diagnostics.AddError("Collision cleanup failed", fmt.Sprintf("API returned status %d; the created identity remains in state for normal cleanup.", cleanup.StatusCode()))
		}
		return
	}
	if !r.refresh(ctx, &data, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Project file not found", "The created file was not returned by the project listing; its identity remains in state for normal cleanup.")
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *ProjectFileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProjectFileResourceModel
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
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *ProjectFileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProjectFileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectID, fileID, err := parseProjectFileID(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid file identity", err.Error())
		return
	}
	_, found := r.metadata(ctx, projectID, fileID, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.Diagnostics.AddError("Project file not found", "The file is no longer owned by this project.")
		return
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(data.ContentBase64.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid file bytes", err.Error())
		return
	}
	if !projectFileEditable(data.Filename.ValueString(), decoded) {
		resp.Diagnostics.AddError("File replacement required", "This content cannot be updated by the text-file API.")
		return
	}
	patch := map[string]any{"content": string(decoded)}
	LogPatch(ctx, "Project file text Update", patch, []AttrSpec{{TFName: "content_base64", JSONName: "content", Kind: Scalar, Sensitive: true}})
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.UpdateSkillSandboxArtifactContentWithBodyWithResponse(ctx, fileID, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Content Update Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	if !r.refresh(ctx, &data, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Project file not found", "The file disappeared after its update.")
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *ProjectFileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProjectFileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectID, fileID, err := parseProjectFileID(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid file identity", err.Error())
		return
	}
	_, found := r.metadata(ctx, projectID, fileID, &resp.Diagnostics)
	if resp.Diagnostics.HasError() || !found {
		return
	}
	result, err := r.client.DeleteSkillSandboxArtifactWithResponse(ctx, fileID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
		resp.Diagnostics.AddError("File Delete Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}
func (r *ProjectFileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	projectID, fileID, err := parseProjectFileID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import identity", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), projectID.String()+":"+fileID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID.String())...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("file_id"), fileID)...)
}

type projectFileWire struct {
	ID           *string `json:"id"`
	Filename     string  `json:"filename"`
	MIMEType     string  `json:"mimeType"`
	SizeBytes    int64   `json:"sizeBytes"`
	CreatedAt    *string `json:"createdAt"`
	DownloadRef  string  `json:"downloadRef"`
	Downloadable bool    `json:"downloadable"`
	ProjectID    *string `json:"projectId"`
	ProjectName  *string `json:"projectName"`
}

func (r *ProjectFileResource) metadata(ctx context.Context, projectID uuid.UUID, fileID string, diags *diag.Diagnostics) (projectFileWire, bool) {
	result, err := r.client.GetProjectFilesWithResponse(ctx, projectID)
	if err != nil {
		diags.AddError("API Error", err.Error())
		return projectFileWire{}, false
	}
	if IsNotFound(result) {
		return projectFileWire{}, false
	}
	if result.JSON200 == nil {
		diags.AddError("Project File List Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return projectFileWire{}, false
	}
	var files []projectFileWire
	if err := json.Unmarshal(result.Body, &files); err != nil {
		diags.AddError("Invalid file metadata", err.Error())
		return projectFileWire{}, false
	}
	for _, file := range files {
		if file.ID != nil && *file.ID == fileID {
			if strings.EqualFold(file.Filename, "instructions.md") {
				diags.AddError("Reserved project file", "Project instructions are managed by the project's instructions attribute.")
				return projectFileWire{}, false
			}
			if file.ProjectID == nil || *file.ProjectID != projectID.String() {
				diags.AddError("Unexpected file scope", "The API returned a file outside the selected project.")
				return projectFileWire{}, false
			}
			return file, true
		}
	}
	return projectFileWire{}, false
}
func (r *ProjectFileResource) refresh(ctx context.Context, data *ProjectFileResourceModel, diags *diag.Diagnostics) bool {
	projectID, fileID, err := parseProjectFileID(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid file identity", err.Error())
		return false
	}
	file, found := r.metadata(ctx, projectID, fileID, diags)
	if !found {
		return false
	}
	raw, err := r.client.GetSkillSandboxArtifactWithResponse(ctx, fileID)
	if err != nil {
		diags.AddError("API Error", err.Error())
		return false
	}
	if raw.StatusCode() != 200 {
		diags.AddError("Project file bytes unavailable", fmt.Sprintf("File metadata remains present; download returned status %d.", raw.StatusCode()))
		return false
	}
	data.ProjectID = types.StringValue(projectID.String())
	data.FileID = types.StringValue(fileID)
	data.Filename = types.StringValue(file.Filename)
	data.MIMEType = types.StringValue(file.MIMEType)
	data.DownloadRef = types.StringValue(file.DownloadRef)
	data.Downloadable = types.BoolValue(file.Downloadable)
	data.ProjectName = types.StringPointerValue(file.ProjectName)
	data.CreatedAt = canonicalTokenTime(file.CreatedAt)
	if data.UploadMIMEType.IsNull() || data.UploadMIMEType.IsUnknown() {
		data.UploadMIMEType = types.StringValue("")
	}
	setProjectFileBytes(data, raw.Body)
	return true
}
func setProjectFileBytes(data *ProjectFileResourceModel, raw []byte) {
	data.ContentBase64 = types.StringValue(base64.StdEncoding.EncodeToString(raw))
	data.SHA256 = types.StringValue(fmt.Sprintf("%x", sha256.Sum256(raw)))
	data.SizeBytes = types.Int64Value(int64(len(raw)))
}
func parseProjectFileID(value string) (uuid.UUID, string, error) {
	project, file, ok := strings.Cut(value, ":")
	if !ok {
		return uuid.Nil, "", fmt.Errorf("use project_uuid:file_uuid")
	}
	projectID, err := uuid.Parse(project)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("use project_uuid:file_uuid")
	}
	fileID, err := uuid.Parse(file)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("use project_uuid:file_uuid")
	}
	return projectID, fileID.String(), nil
}
func projectFileEditable(name string, content []byte) bool {
	extension := strings.ToLower(filepath.Ext(name))
	return (extension == ".md" || extension == ".txt") && utf8.Valid(content) && len(content) <= 1000000
}

type projectFilenameValidator struct{}

func (projectFilenameValidator) Description(context.Context) string {
	return "Use a canonical safe basename of at most 255 UTF-8 bytes, excluding instructions.md."
}
func (v projectFilenameValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (projectFilenameValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	bad := value == "" || value != strings.TrimSpace(value) || strings.HasPrefix(value, ".") || strings.ContainsAny(value, "/\\") || len(value) > 255 || strings.EqualFold(value, "instructions.md")
	for _, r := range value {
		bad = bad || r <= 0x1f || r == 0x7f
	}
	if bad {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid project filename", "Use a safe basename without surrounding whitespace, separators, controls or leading dot; instructions.md belongs to project instructions.")
	}
}

type projectFileBase64Validator struct{}

func (projectFileBase64Validator) Description(context.Context) string {
	return "Use canonical base64 encoding of at most 25 MiB."
}
func (v projectFileBase64Validator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (projectFileBase64Validator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil || base64.StdEncoding.EncodeToString(decoded) != value || len(decoded) > 25*1024*1024 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid project file bytes", "Use canonical base64 encoding within the 25 MiB upload limit.")
	}
}
