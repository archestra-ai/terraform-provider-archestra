package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// Raise the floor only when required API capabilities change. Widen the tested
// interval only after real acceptance and both-scope reconciliation pass.
const minimumAPIVersion = "1.4.0-rc.51"
const testedAPIConstraint = "= 1.4.0-rc.55"

func checkAPICompatibility(ctx context.Context, httpClient *http.Client, baseURL, providerVersion, requestedConstraint string) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var constraint version.Constraints
	if requestedConstraint != "" {
		var err error
		constraint, err = version.NewConstraint(requestedConstraint)
		if err != nil {
			diagnostics.AddError("Invalid Archestra API version constraint", "api_version_constraint must be a valid version constraint, for example = 1.4.0-rc.55 or >= 1.4.0, < 1.5.0.")
			return diagnostics
		}
	}
	rawVersion, err := readAPIVersion(ctx, httpClient, baseURL)
	if err != nil {
		detail := "The provider could not verify the server version through GET /health. No compatibility is established. Check the API URL, connectivity and health endpoint."
		if requestedConstraint != "" {
			diagnostics.AddError("Unable to verify Archestra API version constraint", detail)
		} else {
			diagnostics.AddWarning("Unable to verify Archestra API compatibility", detail)
		}
		return diagnostics
	}
	serverVersion, err := version.NewSemver(rawVersion)
	if err != nil {
		detail := "The health endpoint did not report a semantic version. Custom/development builds have unverified API compatibility."
		if requestedConstraint != "" {
			diagnostics.AddError("Unable to verify Archestra API version constraint", detail)
		} else {
			diagnostics.AddWarning("Unverified Archestra API version", detail)
		}
		return diagnostics
	}
	minimum := version.Must(version.NewSemver(minimumAPIVersion))
	if serverVersion.LessThan(minimum) {
		diagnostics.AddError("Incompatible Archestra API version", fmt.Sprintf("Provider %s requires Archestra >= %s; the server runs %s. Upgrade Archestra or select a compatible older provider. No resource operations will be configured.", providerVersion, minimumAPIVersion, serverVersion))
		return diagnostics
	}
	if constraint != nil && !constraint.Check(serverVersion) {
		diagnostics.AddError("Archestra API version constraint not satisfied", fmt.Sprintf("The server runs %s, which does not satisfy api_version_constraint %q. Select the intended API deployment or deliberately update the constraint after reviewing compatibility.", serverVersion, requestedConstraint))
		return diagnostics
	}
	tested := version.MustConstraints(version.NewConstraint(testedAPIConstraint))
	if !tested.Check(serverVersion) {
		diagnostics.AddWarning("Untested Archestra API version", fmt.Sprintf("Provider %s has verified API compatibility with %s; the server runs %s. Meeting the minimum version does not guarantee compatibility. Pin a tested API release or validate this combination before applying changes.", providerVersion, testedAPIConstraint, serverVersion))
	}
	return diagnostics
}

func readAPIVersion(ctx context.Context, httpClient *http.Client, baseURL string) (string, error) {
	endpoint, err := url.JoinPath(baseURL, "health")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// #nosec G704 -- The practitioner explicitly configures the API destination; no remote response supplies it.
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	// Health is public: never attach the API key or follow redirects to a login
	// page or another host. Reuse the configured transport and its TLS settings.
	probe := *httpClient
	probe.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	// #nosec G704 -- This unauthenticated probe uses the configured API destination and refuses all redirects.
	response, err := probe.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("health status %d", response.StatusCode)
	}
	var health struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&health); err != nil {
		return "", err
	}
	if health.Version == "" || len(health.Version) > 128 {
		return "", fmt.Errorf("missing or oversized version")
	}
	return health.Version, nil
}
