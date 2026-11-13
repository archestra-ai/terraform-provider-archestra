package provider

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The block for the stored type round-trips through the API's `config` union,
// and the other blocks stay null.
func TestBlobProviderConfigRoundTrip(t *testing.T) {
	s3, d := types.ObjectValue(s3BlockType.AttrTypes, map[string]attr.Value{
		"bucket": types.StringValue("acme"), "region": types.StringValue("us-east-1"), "key_prefix": types.StringValue("exports"),
		"endpoint": types.StringNull(), "force_path_style": types.BoolValue(false), "auth_method": types.StringValue("assume_role"),
		"role_arn": types.StringValue("arn:aws:iam::123456789012:role/export"), "external_id": types.StringNull(),
	})
	if d.HasError() {
		t.Fatal(d)
	}
	model := BlobProviderResourceModel{S3: s3, GCS: types.ObjectNull(gcsBlockType.AttrTypes), AzureBlob: types.ObjectNull(azureBlobBlockType.AttrTypes)}
	var diags diag.Diagnostics
	config := blobProviderConfig(model, &diags)
	want := map[string]any{"type": "s3", "bucket": "acme", "region": "us-east-1", "keyPrefix": "exports", "forcePathStyle": false, "authMethod": "assume_role", "roleArn": "arn:aws:iam::123456789012:role/export"}
	if diags.HasError() || !reflect.DeepEqual(config, want) {
		t.Fatalf("config = %#v, diags %v", config, diags)
	}

	creds, d := types.MapValue(types.StringType, map[string]attr.Value{"accessKeyId": types.StringValue("AKIA")})
	if d.HasError() {
		t.Fatal(d)
	}
	state := BlobProviderResourceModel{Credentials: creds}
	body := `{"id":"11111111-1111-4111-8111-111111111111","name":"exports","type":"s3","hasCredentials":true,
		"config":{"type":"s3","bucket":"acme","region":"us-east-1","keyPrefix":"exports","endpoint":null,"forcePathStyle":false,
		"authMethod":"assume_role","roleArn":"arn:aws:iam::123456789012:role/export","externalId":null}}`
	flattenBlobProvider(&state, []byte(body), &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !state.S3.Equal(s3) || !state.GCS.IsNull() || !state.AzureBlob.IsNull() {
		t.Fatalf("blocks = s3 %v gcs %v azure %v", state.S3, state.GCS, state.AzureBlob)
	}
	if !state.Credentials.Equal(creds) || !state.HasCredentials.ValueBool() || state.Type.ValueString() != "s3" {
		t.Fatalf("write-only credentials or computed fields changed: %+v", state)
	}
}

// An unknown provider type from a newer API fails loudly instead of dropping
// the storage settings from state.
func TestBlobProviderRejectsUnknownType(t *testing.T) {
	var diags diag.Diagnostics
	var state BlobProviderResourceModel
	flattenBlobProvider(&state, []byte(`{"id":"x","name":"n","type":"r2","hasCredentials":false,"config":{"type":"r2"}}`), &diags)
	if !diags.HasError() {
		t.Fatal("expected an unsupported-type error")
	}
}

func TestCostExportStartFromReplacement(t *testing.T) {
	for _, tc := range []struct {
		name         string
		state, plan  types.String
		wantReplaced bool
	}{
		{"changed after creation", types.StringValue("2026-01-01T00:00:00Z"), types.StringValue("2026-02-01T00:00:00Z"), true},
		{"unchanged", types.StringValue("2026-01-01T00:00:00Z"), types.StringValue("2026-01-01T00:00:00Z"), false},
		{"set after import", types.StringNull(), types.StringValue("2026-01-01T00:00:00Z"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var resp stringplanmodifier.RequiresReplaceIfFuncResponse
			startFromChanged(context.Background(), planmodifier.StringRequest{StateValue: tc.state, PlanValue: tc.plan}, &resp)
			if resp.RequiresReplace != tc.wantReplaced {
				t.Fatalf("RequiresReplace = %v, want %v", resp.RequiresReplace, tc.wantReplaced)
			}
		})
	}
}

// testAccRequireBlobProviders skips on API releases before blob providers and
// cost exports (1.5.0-rc.30); pull request CI also runs older releases.
func testAccRequireBlobProviders(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		return
	}
	testAccPreCheck(t)
	if !testAccAPIVersionAtLeast(t, "1.5.0-rc.30") {
		t.Skip("Blob providers and cost exports require API 1.5.0-rc.30")
	}
}

// Connection tests are skipped so CI needs no bucket: the API saves the
// provider without writing to it.
func TestAccBlobProviderResource(t *testing.T) {
	testAccRequireBlobProviders(t)
	name := acctest.RandomWithPrefix("tf-acc-blob")
	config := func(name, prefix, auth, creds string) string {
		return fmt.Sprintf(`
resource "archestra_blob_provider" "test" {
  name                 = %q
  skip_connection_test = true
  s3 = {
    bucket      = "tf-acc-bucket"
    region      = "us-east-1"
    key_prefix  = %q
    auth_method = %q
  }
  %s
}

data "archestra_blob_provider" "test" {
  name = archestra_blob_provider.test.name
}`, name, prefix, auth, creds)
	}
	staticKey := `credentials = { accessKeyId = "AKIAEXAMPLE", secretAccessKey = "example-secret" }`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name, "exports", "access_key", staticKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_blob_provider.test", "type", "s3"),
					resource.TestCheckResourceAttr("archestra_blob_provider.test", "has_credentials", "true"),
					resource.TestCheckResourceAttr("archestra_blob_provider.test", "s3.key_prefix", "exports"),
					resource.TestCheckResourceAttr("archestra_blob_provider.test", "s3.force_path_style", "false"),
					resource.TestCheckResourceAttrPair("data.archestra_blob_provider.test", "id", "archestra_blob_provider.test", "id"),
					resource.TestCheckResourceAttr("data.archestra_blob_provider.test", "has_credentials", "true"),
				),
			},
			{
				ResourceName:            "archestra_blob_provider.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"credentials", "skip_connection_test"},
			},
			{
				// Renaming and switching to the platform's credential chain
				// clears the stored static key.
				Config: config(name+"-renamed", "exports/llm", "default_chain", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_blob_provider.test", "name", name+"-renamed"),
					resource.TestCheckResourceAttr("archestra_blob_provider.test", "s3.key_prefix", "exports/llm"),
					resource.TestCheckResourceAttr("archestra_blob_provider.test", "s3.auth_method", "default_chain"),
					resource.TestCheckResourceAttr("archestra_blob_provider.test", "has_credentials", "false"),
				),
			},
		},
	})
}

func TestAccCostExportResource(t *testing.T) {
	testAccRequireBlobProviders(t)
	name := acctest.RandomWithPrefix("tf-acc-cost-export")
	config := func(name, schedule, prefix string) string {
		return fmt.Sprintf(`
resource "archestra_blob_provider" "test" {
  name                 = %[1]q
  skip_connection_test = true
  gcs = {
    bucket      = "tf-acc-bucket"
    auth_method = "application_default"
  }
}

resource "archestra_cost_export" "test" {
  name             = %[1]q
  blob_provider_id = archestra_blob_provider.test.id
  schedule         = %[2]q
  path_prefix      = %[3]q
  # Disabled so the test never writes to the fake bucket.
  enabled          = false
  start_from       = "2026-01-01T00:00:00Z"
}`, name, schedule, prefix)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name, "hourly", "llm-costs"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("archestra_cost_export.test", "blob_provider_id", "archestra_blob_provider.test", "id"),
					resource.TestCheckResourceAttr("archestra_cost_export.test", "format", "focus_csv"),
					resource.TestCheckResourceAttr("archestra_cost_export.test", "schedule", "hourly"),
					resource.TestCheckResourceAttr("archestra_cost_export.test", "enabled", "false"),
				),
			},
			{
				ResourceName:            "archestra_cost_export.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"start_from"},
			},
			{
				Config: config(name, "daily", "llm-costs/daily"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("archestra_cost_export.test", "schedule", "daily"),
					resource.TestCheckResourceAttr("archestra_cost_export.test", "path_prefix", "llm-costs/daily"),
				),
			},
		},
	})
}
