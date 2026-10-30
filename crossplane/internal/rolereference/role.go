// Package rolereference reads role slugs without importing the generated identity
// package, which already depends on teams through service-account ownership.
package rolereference

import (
	"context"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reference"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type role struct{ unstructured.Unstructured }

func (r *role) SetManagementPolicies(value xpv1.ManagementPolicies) {
	_ = fieldpath.Pave(r.Object).SetValue("spec.managementPolicies", value)
}
func (r *role) GetManagementPolicies() xpv1.ManagementPolicies {
	var value xpv1.ManagementPolicies
	_ = fieldpath.Pave(r.Object).GetValueInto("spec.managementPolicies", &value)
	return value
}
func (r *role) SetConditions(values ...xpv1.Condition) {
	var status xpv1.ConditionedStatus
	_ = fieldpath.Pave(r.Object).GetValueInto("status", &status)
	status.SetConditions(values...)
	_ = fieldpath.Pave(r.Object).SetValue("status.conditions", status.Conditions)
}
func (r *role) GetCondition(kind xpv1.ConditionType) xpv1.Condition {
	var status xpv1.ConditionedStatus
	_ = fieldpath.Pave(r.Object).GetValueInto("status", &status)
	return status.GetCondition(kind)
}
func (r *role) DeepCopyObject() runtime.Object { return &role{*r.Unstructured.DeepCopy()} }

type roleList struct{ unstructured.UnstructuredList }

func (r *roleList) DeepCopyObject() runtime.Object { return &roleList{*r.UnstructuredList.DeepCopy()} }
func (r *roleList) GetItems() []resource.Managed {
	out := make([]resource.Managed, len(r.Items))
	for i := range r.Items {
		out[i] = &role{r.Items[i]}
	}
	return out
}

// Target selects the same Role CRD as the generated reference marker.
func Target(group string) reference.To {
	value, list := &role{}, &roleList{}
	value.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: "v1alpha1", Kind: "Role"})
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: "v1alpha1", Kind: "RoleList"})
	return reference.To{Managed: value, List: list}
}

// Slug extracts the immutable role slug, rather than the role's database ID.
func Slug() reference.ExtractValueFn {
	return func(value resource.Managed) string {
		r, ok := value.(*role)
		if !ok {
			return ""
		}
		slug, _, _ := unstructured.NestedString(r.Object, "status", "atProvider", "role")
		return slug
	}
}

type reader struct{ client.Reader }

// Reader keeps all ordinary target reads typed and unwraps only the Role view.
func Reader(value client.Reader) client.Reader { return reader{value} }
func (r reader) Get(ctx context.Context, key client.ObjectKey, value client.Object, opts ...client.GetOption) error {
	if v, ok := value.(*role); ok {
		return r.Reader.Get(ctx, key, &v.Unstructured, opts...)
	}
	return r.Reader.Get(ctx, key, value, opts...)
}
func (r reader) List(ctx context.Context, value client.ObjectList, opts ...client.ListOption) error {
	if v, ok := value.(*roleList); ok {
		return r.Reader.List(ctx, &v.UnstructuredList, opts...)
	}
	return r.Reader.List(ctx, value, opts...)
}

var _ resource.Managed = &role{}
var _ resource.ManagedList = &roleList{}
