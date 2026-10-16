package provider

import (
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

// knowledgeBaseAttrSpec covers the body wire fields shared by
// CreateKnowledgeBase and UpdateKnowledgeBase. Both endpoints make
// every body field optional on the wire (Update declares `name` as
// `z.string().min(1).optional()` and `description` as
// `z.string().nullable().optional()`), so MergePatch's "only-changed-
// fields" output is a valid wire body without finalize hacks.
var knowledgeBaseAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "description", JSONName: "description", Kind: Scalar},
}

func (r *KnowledgeBaseResource) AttrSpecs() []AttrSpec {
	return knowledgeBaseAttrSpec
}

func (r *KnowledgeBaseResource) APIShape() any {
	return client.GetKnowledgeBaseResponse{}
}

// KnownIntentionallySkipped — wire fields not modeled on this resource:
//   - organizationId: ownership metadata. The API key authenticates the
//     org; surfacing the field would create a phantom diff against the
//     resource's implicit scoping.
func (r *KnowledgeBaseResource) KnownIntentionallySkipped() []string {
	return []string{"organizationId"}
}
