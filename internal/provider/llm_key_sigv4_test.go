package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestSigV4RotationIsAtomicAndMetadataDoesNotResendSecrets(t *testing.T) {
	initial := LLMProviderApiKeyResourceModel{AWSAccessKeyID: types.StringValue("synthetic-access"), AWSSecretAccessKey: types.StringValue("synthetic-secret"), AWSSessionToken: types.StringValue("initial-token")}
	for _, tc := range []struct {
		name      string
		token     types.String
		wantPair  bool
		wantToken bool
	}{
		{"metadata", initial.AWSSessionToken, false, false},
		{"token rotation", types.StringValue("rotated-token"), true, true},
		{"token removal", types.StringNull(), true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := initial
			plan.AWSSessionToken = tc.token
			patch := map[string]any{"name": "renamed"}
			prepareSigV4Patch(&plan, &initial, patch)
			_, pair := patch["awsAccessKeyId"]
			_, token := patch["awsSessionToken"]
			if pair != tc.wantPair || token != tc.wantToken {
				t.Fatal("credential rotation did not honor complete-secret replacement")
			}
			if pair && patch["awsSecretAccessKey"] != initial.AWSSecretAccessKey.ValueString() {
				t.Fatal("paired secret missing from rotation")
			}
		})
	}
}
