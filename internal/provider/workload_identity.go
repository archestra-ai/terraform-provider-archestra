package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	workloadIdentityTokenPath = "/api/workload-identity/token"
	tokenExchangeGrantType    = "urn:ietf:params:oauth:grant-type:token-exchange"
	jwtTokenType              = "urn:ietf:params:oauth:token-type:jwt"

	// Exchanged tokens last an hour. Refreshing well before expiry keeps a
	// request that starts just before the deadline from failing mid-apply.
	workloadIdentityRefreshMargin = 5 * time.Minute

	workloadIdentityMinimumAPIVersion = "1.5.0-rc.26"
)

// workloadIdentityTokenSource exchanges a workload's OIDC token for a
// short-lived Archestra service-account token and refreshes it before expiry.
// A token file is re-read on every exchange, so a rotated projected
// Kubernetes token is picked up without restarting Terraform.
type workloadIdentityTokenSource struct {
	httpClient       *http.Client
	baseURL          string
	serviceAccountID string
	oidcToken        string
	oidcTokenFile    string
	now              func() time.Time

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

// Token returns a valid access token, exchanging a new one when the cached
// token is missing or close to expiry.
func (s *workloadIdentityTokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accessToken != "" && s.now().Add(workloadIdentityRefreshMargin).Before(s.expiresAt) {
		return s.accessToken, nil
	}
	subjectToken, err := s.subjectToken()
	if err != nil {
		return "", err
	}
	token, expiresIn, err := exchangeWorkloadIdentityToken(ctx, exchangeWorkloadIdentityTokenInput{
		httpClient:       s.httpClient,
		baseURL:          s.baseURL,
		serviceAccountID: s.serviceAccountID,
		subjectToken:     subjectToken,
	})
	if err != nil {
		return "", err
	}
	s.accessToken = token
	s.expiresAt = s.now().Add(expiresIn)
	return token, nil
}

func (s *workloadIdentityTokenSource) subjectToken() (string, error) {
	if s.oidcToken != "" {
		return s.oidcToken, nil
	}
	data, err := os.ReadFile(s.oidcTokenFile)
	if err != nil {
		return "", fmt.Errorf("cannot read the OIDC token file %q: %w", s.oidcTokenFile, err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("the OIDC token file %q is empty", s.oidcTokenFile)
	}
	return token, nil
}

type exchangeWorkloadIdentityTokenInput struct {
	httpClient       *http.Client
	baseURL          string
	serviceAccountID string
	subjectToken     string
}

func exchangeWorkloadIdentityToken(ctx context.Context, in exchangeWorkloadIdentityTokenInput) (string, time.Duration, error) {
	form := url.Values{
		"grant_type":         {tokenExchangeGrantType},
		"subject_token_type": {jwtTokenType},
		"subject_token":      {in.subjectToken},
		"service_account_id": {in.serviceAccountID},
	}
	endpoint := strings.TrimRight(in.baseURL, "/") + workloadIdentityTokenPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("cannot build the workload identity token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := in.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("workload identity token exchange failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", 0, fmt.Errorf("cannot read the workload identity token response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		var ok struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int64  `json:"expires_in"`
		}
		if err := json.Unmarshal(body, &ok); err != nil || ok.AccessToken == "" || ok.ExpiresIn <= 0 {
			return "", 0, errors.New("the workload identity token response has no access token or lifetime")
		}
		return ok.AccessToken, time.Duration(ok.ExpiresIn) * time.Second, nil
	case resp.StatusCode == http.StatusNotFound:
		return "", 0, fmt.Errorf("this Archestra server does not support workload identity token exchange (it needs Archestra %s or later with an Enterprise license)", workloadIdentityMinimumAPIVersion)
	default:
		var failure struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if json.Unmarshal(body, &failure) == nil && failure.Error != "" {
			return "", 0, fmt.Errorf("workload identity token exchange was refused (HTTP %d, %s): %s", resp.StatusCode, failure.Error, failure.ErrorDescription)
		}
		return "", 0, fmt.Errorf("workload identity token exchange returned HTTP %d", resp.StatusCode)
	}
}
