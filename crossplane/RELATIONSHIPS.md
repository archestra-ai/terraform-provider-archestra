# Managed resource relationships

Reference declarations live in `config/resources.go` and generate both API
scopes. Scalar inputs expose `*Ref` and `*Selector`; collection inputs expose
`*Refs` and `*Selector`. Existing literal identifiers remain supported.

Team hierarchy and role assignments support `parentIdRef` / `parentIdSelector`
and `rolesRefs` / `rolesSelector`. For example:

```yaml
apiVersion: team.archestra.m.crossplane.io/v1alpha1
kind: Team
metadata:
  name: engineering
  namespace: crossplane-system
spec:
  forProvider:
    name: Engineering
    parentIdRef:
      name: company
    rolesRefs:
      - name: project-reader
  providerConfigRef:
    kind: ProviderConfig
    name: default
```

The parent must be an existing Team managed resource and the role must be a
custom Role managed resource in the same API scope. Namespaced references and
selectors default to the requesting resource's namespace; their optional
`namespace` field explicitly selects another namespace. Role references extract `status.atProvider.role`, the immutable
role slug. A Role's external-name/database ID is a different identity and is
not a valid role assignment. ServiceAccount `roleRef` / `roleSelector` uses the
same slug; literal predefined roles and comma-separated role strings remain
available through `role`. Team `roles` can contain literal predefined and
custom slugs.

By default a nonempty literal or previously resolved value is retained even when
a reference is present. Set the reference/selector policy `resolve: Always` to
refresh it, or clear the resolved value to trigger resolution again. An unresolved
Role with no observed `status.atProvider.role` produces a reference error and is
retried after the Role reconciles; its database ID is never substituted.

Omitted collections stay unmanaged; an explicit `roles: []` revokes
assignments. References in `initProvider` follow normal creation-only semantics.
Reference lookup does not authorize an assignment: the platform still validates
role delegation, organization boundaries, and self/cyclic team parent links.
A creation cycle between managed resources cannot be bootstrapped using references;
create the parent first and avoid cyclic hierarchy declarations.

## Coverage audit

The October 2026 audit checked every writable identifier field in every managed
Terraform resource exposed by Crossplane. The added links are:

| Owner | Inputs and targets |
| --- | --- |
| Team | Parent → Team; roles → Role slugs |
| ServiceAccount | Role → Role slug |
| Agent | Model pool → ModelPool; pinned/fast/smart models → Model; allowed/excluded external skills → MCP Server or Plugin |
| Gateway | Environment → Environment; knowledge bases → KnowledgeBase |
| RegistryEntry | Environment → Environment; default dynamic connection → MCP Server; enterprise credential issuer → identity Provider |
| MCP Server | Owning team → Team |
| OAuthClient | Model pool grants → ModelPool |
| Organization Settings | Default agent → Agent; primary/embedding/OCR/reranker provider keys → ModelProvider; default/fast/smart models → Model; default connection gateway → Gateway |

Already configured relationships include service-account/team ownership, OAuth
service-account/billing/provider bindings, project agents/apps, schedules,
Git credentials, skill environments, agent delegations/native skills/provider
keys, model-pool members, share-link members, and knowledge-file directories.

The remaining raw inputs are intentional or deferred:

| Inputs | Reason |
| --- | --- |
| `permissions.subject_id`, default-permission subjects, Limit `entity_id`, VirtualKey `owner_id`, OAuthClient `allowed_gateway_ids` | Polymorphic targets or special principals such as `*`; a single-kind reference would change their meaning. |
| Team members' `user_id`, server `user_id`, SSO `external_group_ids`, GitHub app/installation IDs, OIDC/SAML IDs | External or human identities without a corresponding managed resource. Team member `role` is a member/admin enum, not a custom Role link. |
| Connector IDs/exclusions, tool IDs/exclusions, stored-secret IDs | No corresponding independent managed-resource identity. Tool UUIDs are discovered from server outputs. |
| ModelPool member `model_id`, model `model_id`, identity-provider `provider_id`, OAuth upstream client IDs | Provider-facing identifiers, distinct from Archestra database object IDs. |
| Organization `connection_default_client_id` / JSON shown-client configuration | Connection client identifiers are configuration values, not one unambiguous managed resource type. |
| Agent runtime `credential_id` | Reuse key for runtime-secret declarations; requires a separately designed key/extractor contract. |
| MCP Server `agent_ids` | Valid typed relationship, deferred: generated `mcp` → `agent` imports cycle with existing agent → server refs. Requires separate cycle-safe support; raw IDs remain usable. |

The Team → Role cycle is handled narrowly during generation: identity already
imports Team through service-account ownership. `team_role_references.go` keeps
public ref markers and both existing CRD groups, but reads the Role through an
unstructured projection in `internal/rolereference`. It delegates reference
policy, selectors, controller matching and namespace handling to Crossplane's
standard resolver. No new Kubernetes API or additional permission is introduced.
