package provider

import (
	"encoding/json"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
)

// encodeConnectorConfigValue parses the jsontypes.Normalized JSON
// string into a real JSON object so the wire body is structured.
// MergePatch calls this AFTER it's already decided the field changed
// (string equality), so the encoder doesn't need to worry about diff
// detection. The discriminator `type` field is added later in
// `finalizeKnowledgeConnectorPatch` (it needs `connector_type` from
// the plan, which isn't visible here).
func encodeConnectorConfigValue(v any) any {
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		// Fall back to the raw string so MergePatch's marshal still
		// produces *something* — ValidateConfig already rejected the
		// non-JSON case at plan time, so this is defensive.
		return s
	}
	return parsed
}

// validConnectorTypes is the source-of-truth list driven by the
// generated client. Adding a new connector type backend-side and
// regenerating picks it up automatically; nothing here hardcodes the
// 11 strings.
var validConnectorTypes = []string{
	string(client.CreateConnectorJSONBodyConnectorTypeJira),
	string(client.CreateConnectorJSONBodyConnectorTypeConfluence),
	string(client.CreateConnectorJSONBodyConnectorTypeGithub),
	string(client.CreateConnectorJSONBodyConnectorTypeGitlab),
	string(client.CreateConnectorJSONBodyConnectorTypeServicenow),
	string(client.CreateConnectorJSONBodyConnectorTypeNotion),
	string(client.CreateConnectorJSONBodyConnectorTypeSharepoint),
	string(client.CreateConnectorJSONBodyConnectorTypeGdrive),
	string(client.CreateConnectorJSONBodyConnectorTypeDropbox),
	string(client.CreateConnectorJSONBodyConnectorTypeAsana),
	string(client.CreateConnectorJSONBodyConnectorTypeLinear),
}

// connectorRequiredConfigFields lists the keys each connector type's
// `config` block MUST contain. Catches plan-time typos and missing
// required fields before the apply hits the backend's discriminated-
// union zod validator (which returns a vaguer error). Empty list
// means the backend validates wholly server-side (notion, dropbox,
// linear, etc. accept config with no required keys).
//
// Sourced from `platform/backend/src/types/knowledge-connector.ts`
// (each `<Type>ConfigSchema`'s non-`.optional()` fields). A backend
// change to required fields needs a matching update here.
var connectorRequiredConfigFields = map[string][]string{
	"jira":       {"jiraBaseUrl", "isCloud"},
	"confluence": {"confluenceUrl", "isCloud"},
	"github":     {"githubUrl", "owner"},
	"gitlab":     {"gitlabUrl"},
	"servicenow": {"instanceUrl"},
	"sharepoint": {"tenantId", "siteUrl"},
	"asana":      {"workspaceGid"},
	// notion, gdrive, dropbox, linear — all-optional configs.
}

// validVisibilityValues — mirror the backend's
// KnowledgeSourceVisibilitySchema enum via the generated client.
var validVisibilityValues = []string{
	string(client.CreateConnectorJSONBodyVisibilityOrgWide),
	string(client.CreateConnectorJSONBodyVisibilityTeamScoped),
}

// knowledgeConnectorAttrSpec covers the body fields shared by Create
// and Update.
//
//   - `config` is Scalar with `encodeConnectorConfigValue`: MergePatch
//     detects diffs by comparing the underlying jsontypes.Normalized
//     string, then the encoder parses it into a real JSON object so
//     the wire body is structured. `finalizeKnowledgeConnectorPatch`
//     adds the discriminator `type` field (sourced from
//     `connector_type`) before send.
//   - `knowledge_base_ids` is Kind: List so it serialises naturally
//     on Create (where the wire body accepts it). On Update the wire
//     body has no such field, so `finalizeKnowledgeConnectorPatch`
//     strips it — the resource reconciles via the assign/unassign
//     endpoints in `reconcileKnowledgeBaseIDs` instead.
var knowledgeConnectorAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "description", JSONName: "description", Kind: Scalar},
	{TFName: "visibility", JSONName: "visibility", Kind: Scalar},
	{TFName: "team_ids", JSONName: "teamIds", Kind: List},
	{TFName: "connector_type", JSONName: "connectorType", Kind: Scalar},
	{TFName: "config", JSONName: "config", Kind: Scalar, Encoder: encodeConnectorConfigValue},
	{
		TFName: "credentials", JSONName: "credentials", Kind: AtomicObject, Sensitive: true,
		Children: []AttrSpec{
			{TFName: "email", JSONName: "email", Kind: Scalar},
			{TFName: "api_token", JSONName: "apiToken", Kind: Scalar, Sensitive: true},
		},
	},
	// `schedule` is OmitOnNull because the backend wire is
	// `z.string().optional()` (non-nullable). Removing the HCL
	// attribute after a value was set would otherwise emit `null` and
	// trip "Invalid input: expected string, received null". The
	// backend has no API path to clear `schedule` to null once set; to
	// truly remove a custom cadence, recreate the connector.
	{TFName: "schedule", JSONName: "schedule", Kind: Scalar, OmitOnNull: true},
	{TFName: "enabled", JSONName: "enabled", Kind: Scalar},
	{TFName: "knowledge_base_ids", JSONName: "knowledgeBaseIds", Kind: List},
}

func (r *KnowledgeConnectorResource) AttrSpecs() []AttrSpec {
	return knowledgeConnectorAttrSpec
}

func (r *KnowledgeConnectorResource) APIShape() any {
	return client.GetConnectorResponse{}
}

// KnownIntentionallySkipped — wire fields not modeled on this resource:
//   - organizationId: ownership metadata; implicit in the API key.
//   - checkpoint: backend sync-state bookkeeping (per-connector
//     incremental cursor); changes on every sync and would create
//     constant phantom drift.
//   - totalDocsIngested: runtime metric, not configuration. Expose
//     via a future `data.archestra_connector_runs` if needed.
//   - lastSyncError: companion to last_sync_status; the status enum
//     captures the actionable signal and error text is verbose log
//     content better surfaced through the UI.
func (r *KnowledgeConnectorResource) KnownIntentionallySkipped() []string {
	return []string{
		"organizationId",
		"checkpoint",
		"totalDocsIngested",
		"lastSyncError",
	}
}
