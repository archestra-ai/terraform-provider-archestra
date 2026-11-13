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

// AutoRouterClient bridges the ALPHA auto-routing router API
// (`/api/auto-routing/routers`, Archestra 1.5.0-rc.29 or later). The generated
// client selects operations explicitly and does not include it. Keep transport
// and authentication on the configured client across generated-client updates.
type AutoRouterClient struct{ base *Client }

// AutoRouterTier is one tier as the API returns it: the model row and the
// provider key that serves it.
type AutoRouterTier struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	ModelID  string `json:"modelId"`
	APIKey   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"apiKey"`
}

type AutoRouterTiers struct {
	Fast   *AutoRouterTier `json:"fast"`
	Normal *AutoRouterTier `json:"normal"`
	Smart  *AutoRouterTier `json:"smart"`
}

type AutoRouterDecisionModel struct {
	APIKeyID *string `json:"apiKeyId"`
	Model    string  `json:"model"`
}

type AutoRouter struct {
	ID                string                  `json:"id"`
	Name              string                  `json:"name"`
	Enabled           bool                    `json:"enabled"`
	BudgetUsdPerDay   *float64                `json:"budgetUsdPerDay"`
	BudgetPreset      *string                 `json:"budgetPreset"`
	Tiers             AutoRouterTiers         `json:"tiers"`
	DecisionModel     AutoRouterDecisionModel `json:"decisionModel"`
	UsesPastSessions  bool                    `json:"usesPastSessions"`
	VirtualAPIKeyIDs  []string                `json:"virtualApiKeyIds"`
	Usage             json.RawMessage         `json:"usage"`
	Generation        int64                   `json:"generation"`
	PlannedGeneration int64                   `json:"plannedGeneration"`
	Status            json.RawMessage         `json:"status"`
}

// GetAutoRouterResponse is the API shape the provider's coverage test walks.
type GetAutoRouterResponse struct{ JSON200 *AutoRouter }

// ErrAutoRoutingDisabled reports a deployment without ARCHESTRA_AUTO_ROUTING_ENABLED.
var ErrAutoRoutingDisabled = fmt.Errorf("auto routing is not enabled on this Archestra deployment; set ARCHESTRA_AUTO_ROUTING_ENABLED=true (or ARCHESTRA_BETA=true)")

func NewAutoRouterClient(api *ClientWithResponses) (*AutoRouterClient, error) {
	base, ok := api.ClientInterface.(*Client)
	if !ok {
		return nil, fmt.Errorf("auto routers require the configured HTTP client, got %T", api.ClientInterface)
	}
	return &AutoRouterClient{base: base}, nil
}

// Enabled reports whether the deployment serves the auto-routing API. Every
// auto-routing route answers 404 when the feature is off.
func (c *AutoRouterClient) Enabled(ctx context.Context) (bool, error) {
	status, err := c.request(ctx, http.MethodGet, "api/auto-routing/state", nil, nil)
	if status == http.StatusNotFound {
		return false, nil
	}
	return err == nil, err
}

func (c *AutoRouterClient) List(ctx context.Context) ([]AutoRouter, error) {
	var routers []AutoRouter
	status, err := c.request(ctx, http.MethodGet, "api/auto-routing/routers", nil, &routers)
	if status == http.StatusNotFound {
		return nil, ErrAutoRoutingDisabled
	}
	if err == nil && routers == nil {
		err = fmt.Errorf("auto router list was null; expected an array")
	}
	return routers, err
}

// Get returns nil without an error when the router no longer exists.
func (c *AutoRouterClient) Get(ctx context.Context, id string) (*AutoRouter, error) {
	var router AutoRouter
	status, err := c.request(ctx, http.MethodGet, "api/auto-routing/routers/"+url.PathEscape(id), nil, &router)
	if status == http.StatusNotFound {
		// The feature gate also answers 404; only a live API proves deletion.
		enabled, enabledErr := c.Enabled(ctx)
		if enabledErr != nil {
			return nil, enabledErr
		}
		if !enabled {
			return nil, ErrAutoRoutingDisabled
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &router, nil
}

// Write creates a router when id is empty and otherwise updates it in place.
func (c *AutoRouterClient) Write(ctx context.Context, id string, body io.Reader) (*AutoRouter, error) {
	method, endpoint := http.MethodPost, "api/auto-routing/routers"
	if id != "" {
		method, endpoint = http.MethodPut, endpoint+"/"+url.PathEscape(id)
	}
	var router AutoRouter
	status, err := c.request(ctx, method, endpoint, body, &router)
	if status == http.StatusNotFound && id == "" {
		return nil, ErrAutoRoutingDisabled
	}
	if err != nil {
		return nil, err
	}
	return &router, nil
}

func (c *AutoRouterClient) Delete(ctx context.Context, id string) (int, error) {
	return c.request(ctx, http.MethodDelete, "api/auto-routing/routers/"+url.PathEscape(id), nil, nil)
}

func (c *AutoRouterClient) request(ctx context.Context, method, endpoint string, body io.Reader, result any) (int, error) {
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
		return response.StatusCode, fmt.Errorf("auto router %s returned HTTP %d; verify the auto-routing feature flag, permissions, tier models (tool-capable chat models with one long-context threshold) and provider keys", strings.ToLower(method), response.StatusCode)
	}
	if result != nil {
		if err := json.NewDecoder(response.Body).Decode(result); err != nil {
			return response.StatusCode, fmt.Errorf("invalid auto router response: %w", err)
		}
	}
	return response.StatusCode, nil
}
