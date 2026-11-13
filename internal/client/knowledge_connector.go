package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// KnowledgeConnectorClient covers the knowledge connector endpoints
// (`/api/connectors`). They are not in the generated client, so this bridge
// reuses the configured transport and authentication instead.
type KnowledgeConnectorClient struct{ base *Client }

// KnowledgeConnector is the GET /api/connectors/{id} response. Every wire field
// is listed so the provider's API coverage check sees the full shape.
type KnowledgeConnector struct {
	ID                            string          `json:"id"`
	OrganizationID                string          `json:"organizationId"`
	Name                          string          `json:"name"`
	Description                   *string         `json:"description"`
	ConnectorType                 string          `json:"connectorType"`
	Config                        json.RawMessage `json:"config"`
	Schedule                      string          `json:"schedule"`
	FtsLanguage                   string          `json:"ftsLanguage"`
	PermissionSyncIntervalSeconds int64           `json:"permissionSyncIntervalSeconds"`
	Enabled                       bool            `json:"enabled"`
	SyncPermissionsFromSource     bool            `json:"syncPermissionsFromSource"`
	EnvironmentID                 *string         `json:"environmentId"`
	Labels                        []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"labels"`
	CreatedAt                 string          `json:"createdAt"`
	UpdatedAt                 string          `json:"updatedAt"`
	DeletedAt                 *string         `json:"deletedAt"`
	CreatedBy                 json.RawMessage `json:"createdBy"`
	CreatedByServiceAccountID *string         `json:"createdByServiceAccountId"`
	SecretID                  *string         `json:"secretId"`
	Visibility                string          `json:"visibility"`
	TeamIDs                   []string        `json:"teamIds"`
	Checkpoint                json.RawMessage `json:"checkpoint"`
	ACLConfigEpoch            int64           `json:"aclConfigEpoch"`
	LastSyncAt                *string         `json:"lastSyncAt"`
	LastSyncStatus            *string         `json:"lastSyncStatus"`
	LastSyncError             *string         `json:"lastSyncError"`
	LastPermissionSyncAt      *string         `json:"lastPermissionSyncAt"`
	LastPermissionSyncStatus  *string         `json:"lastPermissionSyncStatus"`
	TotalDocsIngested         int64           `json:"totalDocsIngested"`
}

// GetKnowledgeConnectorResponse mirrors the generated `Get<Op>Response` shape.
type GetKnowledgeConnectorResponse struct{ JSON200 *KnowledgeConnector }

// KnowledgeConnectorError is a non-200 answer. Message is the API's own error
// message; request bodies, which can hold credentials, are never echoed.
type KnowledgeConnectorError struct {
	Status  int
	Method  string
	Message string
}

func (e *KnowledgeConnectorError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("knowledge connector %s returned HTTP %d: %s", e.Method, e.Status, e.Message)
	}
	return fmt.Sprintf("knowledge connector %s returned HTTP %d", e.Method, e.Status)
}

// IsKnowledgeConnectorNotFound reports whether err is an HTTP 404 answer.
func IsKnowledgeConnectorNotFound(err error) bool {
	apiErr, ok := err.(*KnowledgeConnectorError)
	return ok && apiErr.Status == http.StatusNotFound
}

func NewKnowledgeConnectorClient(api *ClientWithResponses) (*KnowledgeConnectorClient, error) {
	base, ok := api.ClientInterface.(*Client)
	if !ok {
		return nil, fmt.Errorf("knowledge connectors require the configured HTTP client, got %T", api.ClientInterface)
	}
	return &KnowledgeConnectorClient{base: base}, nil
}

func (c *KnowledgeConnectorClient) Create(ctx context.Context, body []byte) (*KnowledgeConnector, error) {
	var connector KnowledgeConnector
	if err := c.request(ctx, http.MethodPost, "api/connectors", body, &connector); err != nil {
		return nil, err
	}
	return &connector, nil
}

func (c *KnowledgeConnectorClient) Get(ctx context.Context, id string) (*KnowledgeConnector, error) {
	var connector KnowledgeConnector
	if err := c.request(ctx, http.MethodGet, "api/connectors/"+url.PathEscape(id), nil, &connector); err != nil {
		return nil, err
	}
	return &connector, nil
}

func (c *KnowledgeConnectorClient) Update(ctx context.Context, id string, body []byte) (*KnowledgeConnector, error) {
	var connector KnowledgeConnector
	if err := c.request(ctx, http.MethodPut, "api/connectors/"+url.PathEscape(id), body, &connector); err != nil {
		return nil, err
	}
	return &connector, nil
}

// Delete moves the connector to the trash (a soft delete). Its credential is
// destroyed and its documents stop being searchable.
func (c *KnowledgeConnectorClient) Delete(ctx context.Context, id string) error {
	return c.request(ctx, http.MethodDelete, "api/connectors/"+url.PathEscape(id), nil, nil)
}

// KnowledgeBaseIDs lists the knowledge bases the connector is assigned to.
func (c *KnowledgeConnectorClient) KnowledgeBaseIDs(ctx context.Context, id string) ([]string, error) {
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := c.request(ctx, http.MethodGet, "api/connectors/"+url.PathEscape(id)+"/knowledge-bases", nil, &result); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(result.Data))
	for _, kb := range result.Data {
		ids = append(ids, kb.ID)
	}
	return ids, nil
}

func (c *KnowledgeConnectorClient) AssignKnowledgeBases(ctx context.Context, id string, knowledgeBaseIDs []string) error {
	body, err := json.Marshal(map[string][]string{"knowledgeBaseIds": knowledgeBaseIDs})
	if err != nil {
		return err
	}
	return c.request(ctx, http.MethodPost, "api/connectors/"+url.PathEscape(id)+"/knowledge-bases", body, nil)
}

func (c *KnowledgeConnectorClient) UnassignKnowledgeBase(ctx context.Context, id, knowledgeBaseID string) error {
	return c.request(ctx, http.MethodDelete, "api/connectors/"+url.PathEscape(id)+"/knowledge-bases/"+url.PathEscape(knowledgeBaseID), nil, nil)
}

func (c *KnowledgeConnectorClient) request(ctx context.Context, method, endpoint string, body []byte, result any) error {
	target, err := url.Parse(c.base.Server)
	if err != nil {
		return err
	}
	target, err = target.Parse(endpoint)
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := c.base.applyEditors(ctx, req, nil); err != nil {
		return err
	}
	response, err := c.base.Client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&apiErr)
		return &KnowledgeConnectorError{Status: response.StatusCode, Method: method, Message: apiErr.Error.Message}
	}
	if result != nil {
		if err := json.NewDecoder(response.Body).Decode(result); err != nil {
			return fmt.Errorf("invalid knowledge connector response: %w", err)
		}
	}
	return nil
}
