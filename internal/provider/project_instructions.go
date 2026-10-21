package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (r *ProjectResource) readInstructions(ctx context.Context, data *ProjectResourceModel, diags *diag.Diagnostics) bool {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid Project ID", err.Error())
		return false
	}
	result, err := r.client.GetProjectInstructionsWithResponse(ctx, id)
	if err != nil {
		diags.AddError("Project Instructions Read Failed", err.Error())
		return false
	}
	if result.JSON200 == nil {
		diags.AddError("Project Instructions Read Failed", fmt.Sprintf("API returned status %d; the project was readable, so its state is retained.", result.StatusCode()))
		return false
	}
	data.Instructions = types.StringValue(result.JSON200.Content)
	return true
}
func (r *ProjectResource) writeInstructions(ctx context.Context, data *ProjectResourceModel, diags *diag.Diagnostics) bool {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid Project ID", err.Error())
		return false
	}
	result, err := r.client.SetProjectInstructionsWithResponse(ctx, id, client.SetProjectInstructionsJSONRequestBody{Content: data.Instructions.ValueString()})
	if err != nil {
		diags.AddError("Project Instructions Update Failed", err.Error())
		return false
	}
	if result.JSON200 == nil {
		diags.AddError("Project Instructions Update Failed", fmt.Sprintf("API returned status %d; changing instructions requires project ownership.", result.StatusCode()))
		return false
	}
	return true
}
