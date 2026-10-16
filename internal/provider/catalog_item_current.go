package provider

import (
	"encoding/json"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"time"
)

func currentCatalogAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"environment_id":                    schema.StringAttribute{Optional: true, MarkdownDescription: "Deployment environment for this catalog item."},
		"multitenant":                       schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown(), boolplanmodifier.RequiresReplace()}, MarkdownDescription: "Whether installations share one runtime; immutable after creation."},
		"cloned_from":                       schema.StringAttribute{Optional: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Source catalog item copied when this item is created."},
		"dynamic_connection_mcp_server_id":  schema.StringAttribute{Optional: true, MarkdownDescription: "Default non-personal server connection for dynamic credential resolution."},
		"catalog_item_approval_reason":      schema.StringAttribute{Computed: true, MarkdownDescription: "Catalog item approval reason."},
		"catalog_item_approval_reviewed_at": schema.StringAttribute{Computed: true, MarkdownDescription: "Catalog item approval reviewed at."},
		"catalog_item_approval_reviewed_by": schema.StringAttribute{Computed: true, MarkdownDescription: "Catalog item approval reviewed by."},
		"catalog_item_approval_status":      schema.StringAttribute{Computed: true, MarkdownDescription: "Catalog item approval status."},
		"created_by":                        schema.StringAttribute{Computed: true, MarkdownDescription: "Creator identity metadata as JSON."},
		"created_by_service_account_id":     schema.StringAttribute{Computed: true, MarkdownDescription: "Created by service account id."},
		"deleted_at":                        schema.StringAttribute{Computed: true, MarkdownDescription: "Deleted at."},
		"deployment_name":                   schema.StringAttribute{Computed: true, MarkdownDescription: "Deployment name."},
		"catalog_reinstall_required":        schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether existing shared installations need reinstallation."},
	}
}
func flattenCurrentCatalogFields(data *MCPServerRegistryResourceModel, response *client.GetInternalMcpCatalogItemResponse, diags *diag.Diagnostics) {
	j := response.JSON200
	data.CatalogItemApprovalReason = types.StringPointerValue(j.CatalogItemApprovalReason)
	data.CatalogItemApprovalReviewedBy = types.StringPointerValue(j.CatalogItemApprovalReviewedBy)
	data.CatalogItemApprovalStatus = types.StringPointerValue(j.CatalogItemApprovalStatus)
	data.DeploymentName = types.StringPointerValue(j.DeploymentName)
	data.CatalogReinstallRequired = types.BoolValue(j.CatalogReinstallRequired)
	data.Multitenant = types.BoolValue(j.Multitenant)
	data.CatalogItemApprovalReviewedAt = types.StringNull()
	if j.CatalogItemApprovalReviewedAt != nil {
		data.CatalogItemApprovalReviewedAt = types.StringValue(j.CatalogItemApprovalReviewedAt.Format(time.RFC3339Nano))
	}
	data.DeletedAt = types.StringNull()
	if j.DeletedAt != nil {
		data.DeletedAt = types.StringValue(j.DeletedAt.Format(time.RFC3339Nano))
	}
	data.CreatedBy = types.StringNull()
	if j.CreatedBy != nil {
		encoded, err := json.Marshal(j.CreatedBy)
		if err != nil {
			diags.AddError("Invalid Creator Metadata", "Unable to encode the catalog creator identity.")
			return
		}
		data.CreatedBy = types.StringValue(string(encoded))
	}
	data.ClonedFrom = types.StringNull()
	if j.ClonedFrom != nil {
		data.ClonedFrom = types.StringValue(j.ClonedFrom.String())
	}
	data.CreatedByServiceAccountID = types.StringNull()
	if j.CreatedByServiceAccountId != nil {
		data.CreatedByServiceAccountID = types.StringValue(j.CreatedByServiceAccountId.String())
	}
	data.DynamicConnectionMcpServerID = types.StringNull()
	if j.DynamicConnectionMcpServerId != nil {
		data.DynamicConnectionMcpServerID = types.StringValue(j.DynamicConnectionMcpServerId.String())
	}
	data.EnvironmentID = types.StringNull()
	if j.EnvironmentId != nil {
		data.EnvironmentID = types.StringValue(j.EnvironmentId.String())
	}
}
