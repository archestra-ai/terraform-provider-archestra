package provider

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ssoApiBody is a normalized view of the identity-provider GET/UPDATE/CREATE
// response shapes. The generated client emits three structurally-identical
// anonymous structs (one per endpoint); we JSON-roundtrip into this single
// type so the response→state mapping has a single home.
//
// Field tags match the wire format exactly (camelCase). Optional fields are
// pointer types so we can distinguish "absent" from "zero".
type ssoApiBody struct {
	Id             string  `json:"id"`
	ProviderId     string  `json:"providerId"`
	Domain         string  `json:"domain"`
	DomainVerified *bool   `json:"domainVerified,omitempty"`
	Issuer         string  `json:"issuer"`
	OrganizationId *string `json:"organizationId,omitempty"`
	UserId         *string `json:"userId,omitempty"`

	OidcConfig     *ssoApiOidc           `json:"oidcConfig,omitempty"`
	SamlConfig     *ssoApiSaml           `json:"samlConfig,omitempty"`
	RoleMapping    *ssoApiRoleMapping    `json:"roleMapping,omitempty"`
	TeamSyncConfig *ssoApiTeamSyncConfig `json:"teamSyncConfig,omitempty"`
}

type ssoApiOidc struct {
	Issuer                       string                  `json:"issuer"`
	DiscoveryEndpoint            string                  `json:"discoveryEndpoint"`
	ClientId                     string                  `json:"clientId"`
	ClientSecret                 string                  `json:"clientSecret"`
	AuthorizationEndpoint        *string                 `json:"authorizationEndpoint,omitempty"`
	TokenEndpoint                *string                 `json:"tokenEndpoint,omitempty"`
	UserInfoEndpoint             *string                 `json:"userInfoEndpoint,omitempty"`
	JwksEndpoint                 *string                 `json:"jwksEndpoint,omitempty"`
	Scopes                       *[]string               `json:"scopes,omitempty"`
	Pkce                         bool                    `json:"pkce"`
	OverrideUserInfo             *bool                   `json:"overrideUserInfo,omitempty"`
	SkipDiscovery                *bool                   `json:"skipDiscovery,omitempty"`
	EnableRpInitiatedLogout      *bool                   `json:"enableRpInitiatedLogout,omitempty"`
	Hd                           *string                 `json:"hd,omitempty"`
	TokenEndpointAuthentication  *string                 `json:"tokenEndpointAuthentication,omitempty"`
	Mapping                      *ssoApiOidcMapping      `json:"mapping,omitempty"`
	EnterpriseManagedCredentials *ssoApiEmcCredentials   `json:"enterpriseManagedCredentials,omitempty"`
}

type ssoApiOidcMapping struct {
	Email         *string            `json:"email,omitempty"`
	EmailVerified *string            `json:"emailVerified,omitempty"`
	ExtraFields   *map[string]string `json:"extraFields,omitempty"`
	Id            *string            `json:"id,omitempty"`
	Image         *string            `json:"image,omitempty"`
	Name          *string            `json:"name,omitempty"`
}

type ssoApiEmcCredentials struct {
	ClientAssertionAudience     *string `json:"clientAssertionAudience,omitempty"`
	ClientId                    *string `json:"clientId,omitempty"`
	ClientSecret                *string `json:"clientSecret,omitempty"`
	ExchangeStrategy            *string `json:"exchangeStrategy,omitempty"`
	PrivateKeyId                *string `json:"privateKeyId,omitempty"`
	PrivateKeyPem               *string `json:"privateKeyPem,omitempty"`
	SubjectTokenType            *string `json:"subjectTokenType,omitempty"`
	TokenEndpoint               *string `json:"tokenEndpoint,omitempty"`
	TokenEndpointAuthentication *string `json:"tokenEndpointAuthentication,omitempty"`
}

type ssoApiSaml struct {
	Issuer               string                  `json:"issuer"`
	EntryPoint           string                  `json:"entryPoint"`
	CallbackUrl          string                  `json:"callbackUrl"`
	Cert                 string                  `json:"cert"`
	Audience             *string                 `json:"audience,omitempty"`
	DigestAlgorithm      *string                 `json:"digestAlgorithm,omitempty"`
	IdentifierFormat     *string                 `json:"identifierFormat,omitempty"`
	DecryptionPvk        *string                 `json:"decryptionPvk,omitempty"`
	PrivateKey           *string                 `json:"privateKey,omitempty"`
	SignatureAlgorithm   *string                 `json:"signatureAlgorithm,omitempty"`
	WantAssertionsSigned *bool                   `json:"wantAssertionsSigned,omitempty"`
	IdpMetadata          *ssoApiIdpMetadata      `json:"idpMetadata,omitempty"`
	SpMetadata           *ssoApiSpMetadata       `json:"spMetadata,omitempty"`
	Mapping              *ssoApiSamlMapping      `json:"mapping,omitempty"`
	AdditionalParams     *map[string]interface{} `json:"additionalParams,omitempty"`
}

type ssoApiIdpMetadata struct {
	Cert                 *string                `json:"cert,omitempty"`
	EncPrivateKey        *string                `json:"encPrivateKey,omitempty"`
	EncPrivateKeyPass    *string                `json:"encPrivateKeyPass,omitempty"`
	EntityID             *string                `json:"entityID,omitempty"`
	EntityURL            *string                `json:"entityURL,omitempty"`
	IsAssertionEncrypted *bool                  `json:"isAssertionEncrypted,omitempty"`
	Metadata             *string                `json:"metadata,omitempty"`
	PrivateKey           *string                `json:"privateKey,omitempty"`
	PrivateKeyPass       *string                `json:"privateKeyPass,omitempty"`
	RedirectURL          *string                `json:"redirectURL,omitempty"`
	SingleSignOnService  *[]ssoApiSsoService    `json:"singleSignOnService,omitempty"`
}

type ssoApiSsoService struct {
	Binding  string `json:"binding"`
	Location string `json:"location"`
}

type ssoApiSpMetadata struct {
	Binding              *string `json:"binding,omitempty"`
	EncPrivateKey        *string `json:"encPrivateKey,omitempty"`
	EncPrivateKeyPass    *string `json:"encPrivateKeyPass,omitempty"`
	EntityID             *string `json:"entityID,omitempty"`
	IsAssertionEncrypted *bool   `json:"isAssertionEncrypted,omitempty"`
	Metadata             *string `json:"metadata,omitempty"`
	PrivateKey           *string `json:"privateKey,omitempty"`
	PrivateKeyPass       *string `json:"privateKeyPass,omitempty"`
}

type ssoApiSamlMapping struct {
	Email         *string            `json:"email,omitempty"`
	EmailVerified *string            `json:"emailVerified,omitempty"`
	ExtraFields   *map[string]string `json:"extraFields,omitempty"`
	FirstName     *string            `json:"firstName,omitempty"`
	Id            *string            `json:"id,omitempty"`
	LastName      *string            `json:"lastName,omitempty"`
	Name          *string            `json:"name,omitempty"`
}

type ssoApiRoleMapping struct {
	DefaultRole  *string             `json:"defaultRole,omitempty"`
	SkipRoleSync *bool               `json:"skipRoleSync,omitempty"`
	StrictMode   *bool               `json:"strictMode,omitempty"`
	Rules        *[]ssoApiRoleRule   `json:"rules,omitempty"`
}

type ssoApiRoleRule struct {
	Expression string `json:"expression"`
	Role       string `json:"role"`
}

type ssoApiTeamSyncConfig struct {
	Enabled          *bool   `json:"enabled,omitempty"`
	GroupsExpression *string `json:"groupsExpression,omitempty"`
}

// mapSsoProviderResponse populates `target` from the API response body, which
// is JSON-roundtripped through ssoApiBody to bridge the three generated
// response types (Get/Create/Update) into a single mapping path.
//
// Drift-honest reads (Decision A7): every field — including sensitive ones —
// is populated from the API response. The backend's identity-provider admin
// endpoint returns full secrets (no redaction; verified at
// platform/backend/src/models/identity-provider.ee.ts:496-529), so prior-state
// preservation is unnecessary and would mask real drift. Plan-output masking
// is handled by `Sensitive: true` on the schema attribute.
//
// `populateRoleMapping` and `populateTeamSync` are gated independently because
// the backend's zod for these is `.optional()` (not `.nullable()`), so the
// merge-patch send-side declares them OmitOnNull — i.e. removing the block
// from HCL doesn't clear the row server-side. Pulling those fields back into
// state when the user dropped them from HCL would cause a spurious "remove"
// plan after refresh. Caller passes `true` only when the field is present in
// HCL (Update with non-nil block) or during import (state is empty and we
// need to capture everything).
func mapSsoProviderResponse(rawBody any, target *SsoProviderResourceModel, populateRoleMapping, populateTeamSync bool) error {
	raw, err := json.Marshal(rawBody)
	if err != nil {
		return fmt.Errorf("marshal sso response: %w", err)
	}
	var api ssoApiBody
	if err := json.Unmarshal(raw, &api); err != nil {
		return fmt.Errorf("unmarshal sso response: %w", err)
	}

	target.ID = types.StringValue(api.Id)
	target.ProviderID = types.StringValue(api.ProviderId)
	target.Domain = types.StringValue(api.Domain)
	target.Issuer = types.StringValue(api.Issuer)
	target.DomainVerified = boolValueOrNull(api.DomainVerified)
	target.OrganizationID = stringValueOrNull(api.OrganizationId)
	target.UserID = stringValueOrNull(api.UserId)

	if api.OidcConfig != nil {
		target.OidcConfig = mapSsoOidcConfig(api.OidcConfig)
	} else {
		target.OidcConfig = nil
	}

	if api.SamlConfig != nil {
		target.SamlConfig = mapSsoSamlConfig(api.SamlConfig)
	} else {
		target.SamlConfig = nil
	}

	if populateRoleMapping && api.RoleMapping != nil {
		target.RoleMapping = mapSsoRoleMapping(api.RoleMapping)
	} else if populateRoleMapping {
		target.RoleMapping = nil
	}
	// else: leave target.RoleMapping untouched (user didn't opt into managing it)

	if populateTeamSync && api.TeamSyncConfig != nil {
		target.TeamSyncConfig = mapSsoTeamSyncConfig(api.TeamSyncConfig)
	} else if populateTeamSync {
		target.TeamSyncConfig = nil
	}
	// else: same rationale as RoleMapping above.

	return nil
}

func mapSsoOidcConfig(o *ssoApiOidc) *OidcConfigModel {
	out := &OidcConfigModel{
		Issuer:                      types.StringValue(o.Issuer),
		DiscoveryEndpoint:           types.StringValue(o.DiscoveryEndpoint),
		ClientID:                    types.StringValue(o.ClientId),
		ClientSecret:                types.StringValue(o.ClientSecret),
		Pkce:                        types.BoolValue(o.Pkce),
		AuthorizationEndpoint:       stringValueOrNull(o.AuthorizationEndpoint),
		TokenEndpoint:               stringValueOrNull(o.TokenEndpoint),
		UserInfoEndpoint:            stringValueOrNull(o.UserInfoEndpoint),
		JwksEndpoint:                stringValueOrNull(o.JwksEndpoint),
		OverrideUserInfo:            boolValueOrNull(o.OverrideUserInfo),
		SkipDiscovery:               boolValueOrNull(o.SkipDiscovery),
		EnableRpInitiatedLogout:     boolValueOrNull(o.EnableRpInitiatedLogout),
		Hd:                          stringValueOrNull(o.Hd),
		TokenEndpointAuthentication: stringValueOrNull(o.TokenEndpointAuthentication),
	}
	if o.Scopes != nil {
		scopes := make([]types.String, len(*o.Scopes))
		for i, s := range *o.Scopes {
			scopes[i] = types.StringValue(s)
		}
		out.Scopes = scopes
	}
	if o.Mapping != nil {
		out.Mapping = &OidcMappingModel{
			Email:         stringValueOrNull(o.Mapping.Email),
			EmailVerified: stringValueOrNull(o.Mapping.EmailVerified),
			ExtraFields:   mapStringToTypes(o.Mapping.ExtraFields),
			ID:            stringValueOrNull(o.Mapping.Id),
			Image:         stringValueOrNull(o.Mapping.Image),
			Name:          stringValueOrNull(o.Mapping.Name),
		}
	}
	if o.EnterpriseManagedCredentials != nil {
		c := o.EnterpriseManagedCredentials
		out.EnterpriseManagedCredentials = &EnterpriseManagedCredentialsModel{
			ExchangeStrategy:            stringValueOrNull(c.ExchangeStrategy),
			ClientID:                    stringValueOrNull(c.ClientId),
			ClientSecret:                stringValueOrNull(c.ClientSecret),
			TokenEndpoint:               stringValueOrNull(c.TokenEndpoint),
			TokenEndpointAuthentication: stringValueOrNull(c.TokenEndpointAuthentication),
			PrivateKeyPem:               stringValueOrNull(c.PrivateKeyPem),
			PrivateKeyID:                stringValueOrNull(c.PrivateKeyId),
			ClientAssertionAudience:     stringValueOrNull(c.ClientAssertionAudience),
			SubjectTokenType:            stringValueOrNull(c.SubjectTokenType),
		}
	}
	return out
}

func mapSsoSamlConfig(s *ssoApiSaml) *SamlConfigModel {
	out := &SamlConfigModel{
		Issuer:               types.StringValue(s.Issuer),
		EntryPoint:           types.StringValue(s.EntryPoint),
		CallbackURL:          types.StringValue(s.CallbackUrl),
		Cert:                 types.StringValue(s.Cert),
		Audience:             stringValueOrNull(s.Audience),
		DigestAlgorithm:      stringValueOrNull(s.DigestAlgorithm),
		IdentifierFormat:     stringValueOrNull(s.IdentifierFormat),
		DecryptionPvk:        stringValueOrNull(s.DecryptionPvk),
		PrivateKey:           stringValueOrNull(s.PrivateKey),
		SignatureAlgorithm:   stringValueOrNull(s.SignatureAlgorithm),
		WantAssertionsSigned: boolValueOrNull(s.WantAssertionsSigned),
		AdditionalParams:     encodeAdditionalParams(s.AdditionalParams),
	}
	if s.IdpMetadata != nil {
		idp := s.IdpMetadata
		out.IdpMetadata = &SamlIdpMetadata{
			Cert:                 stringValueOrNull(idp.Cert),
			EncPrivateKey:        stringValueOrNull(idp.EncPrivateKey),
			EncPrivateKeyPass:    stringValueOrNull(idp.EncPrivateKeyPass),
			EntityID:             stringValueOrNull(idp.EntityID),
			EntityURL:            stringValueOrNull(idp.EntityURL),
			IsAssertionEncrypted: boolValueOrNull(idp.IsAssertionEncrypted),
			Metadata:             stringValueOrNull(idp.Metadata),
			PrivateKey:           stringValueOrNull(idp.PrivateKey),
			PrivateKeyPass:       stringValueOrNull(idp.PrivateKeyPass),
			RedirectURL:          stringValueOrNull(idp.RedirectURL),
		}
		if idp.SingleSignOnService != nil {
			services := make([]SsoService, len(*idp.SingleSignOnService))
			for i, svc := range *idp.SingleSignOnService {
				services[i] = SsoService{
					Binding:  types.StringValue(svc.Binding),
					Location: types.StringValue(svc.Location),
				}
			}
			out.IdpMetadata.SingleSignOnService = services
		}
	}
	if s.SpMetadata != nil && (s.SpMetadata.Metadata != nil || s.SpMetadata.EntityID != nil) {
		sp := s.SpMetadata
		out.SpMetadata = &SamlSpMetadata{
			Binding:              stringValueOrNull(sp.Binding),
			EncPrivateKey:        stringValueOrNull(sp.EncPrivateKey),
			EncPrivateKeyPass:    stringValueOrNull(sp.EncPrivateKeyPass),
			EntityID:             stringValueOrNull(sp.EntityID),
			IsAssertionEncrypted: boolValueOrNull(sp.IsAssertionEncrypted),
			Metadata:             stringValueOrNull(sp.Metadata),
			PrivateKey:           stringValueOrNull(sp.PrivateKey),
			PrivateKeyPass:       stringValueOrNull(sp.PrivateKeyPass),
		}
	}
	if s.Mapping != nil {
		out.Mapping = &SamlMappingModel{
			Email:         stringValueOrNull(s.Mapping.Email),
			EmailVerified: stringValueOrNull(s.Mapping.EmailVerified),
			ExtraFields:   mapStringToTypes(s.Mapping.ExtraFields),
			FirstName:     stringValueOrNull(s.Mapping.FirstName),
			ID:            stringValueOrNull(s.Mapping.Id),
			LastName:      stringValueOrNull(s.Mapping.LastName),
			Name:          stringValueOrNull(s.Mapping.Name),
		}
	}
	return out
}

func mapSsoRoleMapping(r *ssoApiRoleMapping) *RoleMappingModel {
	out := &RoleMappingModel{
		DefaultRole:  stringValueOrNull(r.DefaultRole),
		SkipRoleSync: boolValueOrNull(r.SkipRoleSync),
		StrictMode:   boolValueOrNull(r.StrictMode),
	}
	if r.Rules != nil {
		rules := make([]RoleRuleModel, len(*r.Rules))
		for i, rule := range *r.Rules {
			rules[i] = RoleRuleModel{
				Expression: types.StringValue(rule.Expression),
				Role:       types.StringValue(rule.Role),
			}
		}
		out.Rules = rules
	}
	return out
}

func mapSsoTeamSyncConfig(t *ssoApiTeamSyncConfig) *TeamSyncConfigModel {
	return &TeamSyncConfigModel{
		Enabled:          boolValueOrNull(t.Enabled),
		GroupsExpression: stringValueOrNull(t.GroupsExpression),
	}
}

// encodeAdditionalParamsMap normalizes a polymorphic-key map into the
// jsontypes.Normalized HCL-side representation. Mirrors encodeAdditionalParams
// but takes the JSON-roundtripped pointer directly.
//
// Returns a null Normalized when the input is nil; otherwise marshals the
// map into a stable JSON string. (Note: encodeAdditionalParams already
// performs this; this signature is kept for future use.)
var _ = jsontypes.Normalized{}
