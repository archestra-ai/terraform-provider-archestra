package clients

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	clusterenvironment "github.com/archestra-ai/provider-archestra/apis/cluster/environment/v1alpha1"
	clusterconfig "github.com/archestra-ai/provider-archestra/apis/cluster/v1beta1"
	namespacedenvironment "github.com/archestra-ai/provider-archestra/apis/namespaced/environment/v1alpha1"
	namespacedconfig "github.com/archestra-ai/provider-archestra/apis/namespaced/v1beta1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInjectedIdentityExchangesOnceAcrossScopes(t *testing.T) {
	var exchanges atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exchanges.Add(1)
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.URL.Path != workloadIdentityExchangePath || r.PostForm.Get("service_account_id") != "sa-fixture" || r.PostForm.Get("subject_token") != "projected-token" {
			t.Errorf("unexpected exchange %s %v", r.URL.Path, r.PostForm)
		}
		_, _ = w.Write([]byte(`{"access_token":"exchanged-token","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("projected-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARCHESTRA_BASE_URL", server.URL)
	useFreshWorkloadIdentityCache(t, server.Client(), time.Now)

	for _, kind := range []string{"legacy", "namespaced", "cluster-config"} {
		t.Run(kind, func(t *testing.T) {
			spec := map[string]any{
				"credentials":      map[string]any{"source": "InjectedIdentity"},
				"workloadIdentity": map[string]any{"serviceAccountId": "sa-fixture", "tokenPath": tokenPath},
			}
			crClient, mg := setupFixture(t, kind, spec)
			result, err := TerraformSetupBuilder("1.5.7", "archestra-ai/archestra", "3.0.0")(t.Context(), crClient, mg)
			if err != nil {
				t.Fatal(err)
			}
			if result.Configuration["api_key"] != "exchanged-token" || result.Configuration["base_url"] != server.URL || len(result.Configuration) != 2 {
				t.Fatalf("configuration = %v", result.Configuration)
			}
		})
	}
	if exchanges.Load() != 1 {
		t.Fatalf("exchanged %d times, want one shared exchange while the token is fresh", exchanges.Load())
	}
}

func TestInjectedIdentityRefreshesNearExpiryAndOnRotation(t *testing.T) {
	var exchanges atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		exchanges.Add(1)
		_, _ = w.Write([]byte(`{"access_token":"t","expires_in":3600}`))
	}))
	defer server.Close()
	now := time.Unix(1_000_000, 0)
	cache := useFreshWorkloadIdentityCache(t, server.Client(), func() time.Time { return now })
	tokenPath := filepath.Join(t.TempDir(), "token")
	spec := &namespacedconfig.WorkloadIdentity{ServiceAccountID: "sa", TokenPath: tokenPath}
	write := func(v string) {
		if err := os.WriteFile(tokenPath, []byte(v), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	get := func() {
		if _, err := cache.workloadIdentityConfiguration(t.Context(), spec, server.URL); err != nil {
			t.Fatal(err)
		}
	}

	write("a")
	get()
	get()
	write("b") // kubelet rotated the projected token
	get()
	now = now.Add(time.Hour - workloadIdentityRefreshMargin)
	get()
	if exchanges.Load() != 3 {
		t.Fatalf("exchanged %d times, want 3 (first, rotation, near expiry)", exchanges.Load())
	}
}

func TestInjectedIdentityConfigurationErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"No federated credential matches."}`))
	}))
	defer server.Close()
	cache := useFreshWorkloadIdentityCache(t, server.Client(), time.Now)
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("jwt"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		spec    *namespacedconfig.WorkloadIdentity
		baseURL string
		want    string
	}{
		{"no spec", nil, server.URL, "spec.workloadIdentity.serviceAccountId"},
		{"no base url", &namespacedconfig.WorkloadIdentity{ServiceAccountID: "sa", TokenPath: tokenPath}, "", "ARCHESTRA_BASE_URL"},
		{"no token file", &namespacedconfig.WorkloadIdentity{ServiceAccountID: "sa", TokenPath: tokenPath + "-missing"}, server.URL, "DeploymentRuntimeConfig"},
		{"refused", &namespacedconfig.WorkloadIdentity{ServiceAccountID: "sa", TokenPath: tokenPath}, server.URL, "invalid_grant): No federated credential matches."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cache.workloadIdentityConfiguration(t.Context(), tc.spec, tc.baseURL)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func useFreshWorkloadIdentityCache(t *testing.T, httpClient *http.Client, now func() time.Time) *workloadIdentityTokens {
	t.Helper()
	previous := sharedWorkloadIdentityTokens
	sharedWorkloadIdentityTokens = &workloadIdentityTokens{httpClient: httpClient, now: now, entries: map[string]cachedWorkloadToken{}}
	t.Cleanup(func() { sharedWorkloadIdentityTokens = previous })
	return sharedWorkloadIdentityTokens
}

func setupFixture(t *testing.T, kind string, spec map[string]any) (client.Client, resource.Managed) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, clusterconfig.SchemeBuilder.AddToScheme, namespacedconfig.SchemeBuilder.AddToScheme, clusterenvironment.AddToScheme, namespacedenvironment.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	const namespace = "fixture"
	var pc client.Object
	var mg resource.Managed
	pcMeta := map[string]any{"name": "fixture"}
	ref := map[string]string{"name": "fixture"}
	switch kind {
	case "legacy":
		pc = &clusterconfig.ProviderConfig{}
		mg = &clusterenvironment.Environment{}
	case "namespaced":
		pc = &namespacedconfig.ProviderConfig{}
		mg = &namespacedenvironment.Environment{}
		pcMeta["namespace"] = namespace
		ref["kind"] = "ProviderConfig"
	case "cluster-config":
		pc = &namespacedconfig.ClusterProviderConfig{}
		mg = &namespacedenvironment.Environment{}
		ref["kind"] = "ClusterProviderConfig"
	}
	mustUnmarshal(t, map[string]any{"metadata": pcMeta, "spec": spec}, pc)
	mustUnmarshal(t, map[string]any{"metadata": map[string]string{"name": "managed-fixture", "namespace": namespace, "uid": "fixture-uid"}, "spec": map[string]any{"providerConfigRef": ref}}, mg)
	if kind == "legacy" {
		mg.SetNamespace("")
	}
	mg.SetUID(types.UID("fixture-uid"))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(pc, mg).Build(), mg
}

func mustUnmarshal(t *testing.T, value any, into any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatal(err)
	}
}
