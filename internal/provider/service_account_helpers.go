package provider

import "github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"

var serviceAccountAttrSpec = []AttrSpec{
	initialGrantsAttrSpec,
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "icon", JSONName: "icon", Kind: Scalar},
	{TFName: "role", JSONName: "role", Kind: Scalar},
	{TFName: "disabled", JSONName: "disabled", Kind: Scalar, OmitOnNull: true},
	{TFName: "labels", JSONName: "labels", Kind: Set, OmitOnNull: true, Children: []AttrSpec{{TFName: "key", JSONName: "key", Kind: Scalar}, {TFName: "value", JSONName: "value", Kind: Scalar}}},
}

func (r *ServiceAccountResource) AttrSpecs() []AttrSpec { return serviceAccountAttrSpec }
func (r *ServiceAccountResource) APIShape() any         { return client.GetServiceAccountResponse{} }

// Tokens have their own resource; creator and organization metadata are authentication-owned.
func (r *ServiceAccountResource) KnownIntentionallySkipped() []string {
	return []string{"tokens", "organizationId"}
}
