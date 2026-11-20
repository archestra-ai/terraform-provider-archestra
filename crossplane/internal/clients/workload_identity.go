package clients

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	namespacedv1beta1 "github.com/archestra-ai/provider-archestra/apis/namespaced/v1beta1"
)

const (
	defaultWorkloadIdentityTokenPath = "/var/run/secrets/archestra/serviceaccount/token"
	workloadIdentityExchangePath     = "/api/workload-identity/token"
	// Upjet runs the Terraform CLI once per reconcile, so the exchange lives
	// here in the long-running controller and the Terraform provider only
	// receives the resulting token. Refresh well before the one-hour expiry so
	// a long apply never starts with a nearly expired token.
	workloadIdentityRefreshMargin = 15 * time.Minute
)

// workloadIdentityTokens caches exchanged tokens per base URL, service account
// and OIDC token, so reconciles share one exchange until it nears expiry. A
// rotated projected token changes the key and triggers a new exchange.
type workloadIdentityTokens struct {
	httpClient *http.Client
	now        func() time.Time

	mu      sync.Mutex
	entries map[string]cachedWorkloadToken
}

type cachedWorkloadToken struct {
	token     string
	expiresAt time.Time
}

var sharedWorkloadIdentityTokens = &workloadIdentityTokens{
	httpClient: &http.Client{Timeout: 30 * time.Second},
	now:        time.Now,
	entries:    map[string]cachedWorkloadToken{},
}

// workloadIdentityConfiguration returns the Terraform provider configuration
// for an InjectedIdentity ProviderConfig: the base URL and an exchanged
// service account token in place of an API key.
func (c *workloadIdentityTokens) workloadIdentityConfiguration(ctx context.Context, spec *namespacedv1beta1.WorkloadIdentity, baseURL string) (map[string]any, error) {
	if spec == nil || spec.ServiceAccountID == "" {
		return nil, errors.New("credentials source InjectedIdentity needs spec.workloadIdentity.serviceAccountId")
	}
	if baseURL == "" {
		return nil, errors.New("credentials source InjectedIdentity needs the ARCHESTRA_BASE_URL environment variable on the provider pod")
	}
	tokenPath := spec.TokenPath
	if tokenPath == "" {
		tokenPath = defaultWorkloadIdentityTokenPath
	}
	raw, err := os.ReadFile(tokenPath) //nolint:gosec // the path is the operator-configured projected token location
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read the provider pod's OIDC token at %s; project a service account token there with a DeploymentRuntimeConfig", tokenPath)
	}
	subjectToken := strings.TrimSpace(string(raw))
	if subjectToken == "" {
		return nil, errors.Errorf("the OIDC token file %s is empty", tokenPath)
	}
	token, err := c.token(ctx, baseURL, spec.ServiceAccountID, subjectToken)
	if err != nil {
		return nil, err
	}
	return map[string]any{"api_key": token, "base_url": baseURL}, nil
}

func (c *workloadIdentityTokens) token(ctx context.Context, baseURL, serviceAccountID, subjectToken string) (string, error) {
	sum := sha256.Sum256([]byte(subjectToken))
	key := baseURL + "\x00" + serviceAccountID + "\x00" + hex.EncodeToString(sum[:])

	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if entry, ok := c.entries[key]; ok && now.Add(workloadIdentityRefreshMargin).Before(entry.expiresAt) {
		return entry.token, nil
	}
	token, expiresIn, err := c.exchange(ctx, baseURL, serviceAccountID, subjectToken)
	if err != nil {
		return "", err
	}
	for k, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, k)
		}
	}
	c.entries[key] = cachedWorkloadToken{token: token, expiresAt: now.Add(expiresIn)}
	return token, nil
}

func (c *workloadIdentityTokens) exchange(ctx context.Context, baseURL, serviceAccountID, subjectToken string) (string, time.Duration, error) {
	form := url.Values{
		"grant_type":         {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"subject_token_type": {"urn:ietf:params:oauth:token-type:jwt"},
		"subject_token":      {subjectToken},
		"service_account_id": {serviceAccountID},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+workloadIdentityExchangePath, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, errors.Wrap(err, "cannot build the workload identity token request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, errors.Wrap(err, "workload identity token exchange failed")
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", 0, errors.Wrap(err, "cannot read the workload identity token response")
	}
	if resp.StatusCode == http.StatusOK {
		var ok struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int64  `json:"expires_in"`
		}
		if err := json.Unmarshal(body, &ok); err != nil || ok.AccessToken == "" || ok.ExpiresIn <= 0 {
			return "", 0, errors.New("the workload identity token response has no access token or lifetime")
		}
		return ok.AccessToken, time.Duration(ok.ExpiresIn) * time.Second, nil
	}
	var failure struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(body, &failure) == nil && failure.Error != "" {
		return "", 0, fmt.Errorf("workload identity token exchange was refused (HTTP %d, %s): %s", resp.StatusCode, failure.Error, failure.ErrorDescription)
	}
	return "", 0, fmt.Errorf("workload identity token exchange returned HTTP %d", resp.StatusCode)
}
