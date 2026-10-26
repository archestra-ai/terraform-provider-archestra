package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ModelPoolClient bridges the beta API from Archestra PR #9681 (bbb3c021),
// which is newer than the pinned generated client. Keep auth and transport
// on the configured client; codegen updates must not remove this extension.
type ModelPoolClient struct{ base *Client }

type ModelPoolMember struct {
	ProviderAPIKeyID string `json:"providerApiKeyId"`
	ModelID          string `json:"modelId"`
	Priority         int64  `json:"priority"`
	Weight           int64  `json:"weight"`
	Enabled          bool   `json:"enabled"`
}

type ModelPool struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	OrganizationID   string            `json:"organizationId"`
	CreatedAt        string            `json:"createdAt"`
	UpdatedAt        string            `json:"updatedAt"`
	Members          []ModelPoolMember `json:"members"`
	VirtualAPIKeyIDs []string          `json:"virtualApiKeyIds"`
}

type GetModelPoolsResponse struct{ JSON200 *[]ModelPool }

func NewModelPoolClient(api *ClientWithResponses) (*ModelPoolClient, error) {
	base, ok := api.ClientInterface.(*Client)
	if !ok {
		return nil, fmt.Errorf("model pools require the configured HTTP client, got %T", api.ClientInterface)
	}
	return &ModelPoolClient{base: base}, nil
}

func (c *ModelPoolClient) List(ctx context.Context) ([]ModelPool, error) {
	var pools []ModelPool
	_, err := c.request(ctx, http.MethodGet, "api/model-pools", nil, &pools)
	if err == nil && pools == nil {
		err = fmt.Errorf("model pool list was null; expected an array")
	}
	return pools, err
}

func (c *ModelPoolClient) Write(ctx context.Context, id string, body io.Reader) (*ModelPool, error) {
	method, endpoint := http.MethodPost, "api/model-pools"
	if id != "" {
		method, endpoint = http.MethodPut, endpoint+"/"+url.PathEscape(id)
	}
	var pool ModelPool
	_, err := c.request(ctx, method, endpoint, body, &pool)
	if err != nil {
		return nil, err
	}
	return &pool, nil
}

func (c *ModelPoolClient) Delete(ctx context.Context, id string) (int, error) {
	return c.request(ctx, http.MethodDelete, "api/model-pools/"+url.PathEscape(id), nil, nil)
}

func (c *ModelPoolClient) request(ctx context.Context, method, endpoint string, body io.Reader, result any) (int, error) {
	target, err := url.Parse(c.base.Server)
	if err != nil {
		return 0, err
	}
	target, err = target.Parse(endpoint)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := c.base.applyEditors(ctx, req, nil); err != nil {
		return 0, err
	}
	response, err := c.base.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		// Never echo upstream bodies, which can include credential material.
		return response.StatusCode, fmt.Errorf("model pool %s returned HTTP %d; verify API support, model-pools feature flag, permissions and references", strings.ToLower(method), response.StatusCode)
	}
	if result != nil {
		if err := json.NewDecoder(response.Body).Decode(result); err != nil {
			return response.StatusCode, fmt.Errorf("invalid model pool response: %w", err)
		}
	}
	return response.StatusCode, nil
}
