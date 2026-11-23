package schemaregistry

import (
	"context"
	"os"
	"testing"

	"github.com/riferrei/srclient"

	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories serves the framework provider for acceptance tests.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"schemaregistry": providerserver.NewProtocol6WithError(New("test")()),
}

// TestProvider validates that the provider schema builds without error.
func TestProvider(t *testing.T) {
	resp := fwprovider.SchemaResponse{}
	New("test")().Schema(context.Background(), fwprovider.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("provider schema diagnostics: %+v", resp.Diagnostics)
	}
}

func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests skipped unless env 'TF_ACC' set")
	}
	if v := os.Getenv("SCHEMA_REGISTRY_URL"); v == "" {
		t.Fatal("SCHEMA_REGISTRY_URL must be set for acceptance tests")
	}

	hasBasicAuth := os.Getenv("SCHEMA_REGISTRY_USERNAME") != "" && os.Getenv("SCHEMA_REGISTRY_PASSWORD") != ""
	hasBearerToken := os.Getenv("SCHEMA_REGISTRY_BEARER_TOKEN") != ""
	hasOAuth2 := os.Getenv("SCHEMA_REGISTRY_OAUTH2_TOKEN_URL") != "" &&
		os.Getenv("SCHEMA_REGISTRY_OAUTH2_CLIENT_ID") != "" &&
		os.Getenv("SCHEMA_REGISTRY_OAUTH2_CLIENT_SECRET") != ""

	if !hasBasicAuth && !hasBearerToken && !hasOAuth2 {
		t.Fatal("Either SCHEMA_REGISTRY_USERNAME and SCHEMA_REGISTRY_PASSWORD, or SCHEMA_REGISTRY_BEARER_TOKEN, or SCHEMA_REGISTRY_OAUTH2_TOKEN_URL/CLIENT_ID/CLIENT_SECRET must be set for acceptance tests")
	}
}

// testAccClient returns a Schema Registry client configured from the
// acceptance-test environment, for tests that seed data directly.
func testAccClient(t *testing.T) *srclient.SchemaRegistryClient {
	t.Helper()
	testAccPreCheck(t)

	client := srclient.CreateSchemaRegistryClient(os.Getenv("SCHEMA_REGISTRY_URL"))
	if username, password := os.Getenv("SCHEMA_REGISTRY_USERNAME"), os.Getenv("SCHEMA_REGISTRY_PASSWORD"); username != "" && password != "" {
		client.SetCredentials(username, password)
	}
	if token := os.Getenv("SCHEMA_REGISTRY_BEARER_TOKEN"); token != "" {
		client.SetBearerToken(token)
	}
	tokenURL := os.Getenv("SCHEMA_REGISTRY_OAUTH2_TOKEN_URL")
	clientID := os.Getenv("SCHEMA_REGISTRY_OAUTH2_CLIENT_ID")
	clientSecret := os.Getenv("SCHEMA_REGISTRY_OAUTH2_CLIENT_SECRET")
	if tokenURL != "" && clientID != "" && clientSecret != "" {
		token, err := getToken(tokenURL, clientID, clientSecret, nil)
		if err != nil {
			t.Fatalf("Failed to get OAuth2 token: %s", err)
		}
		client.SetBearerToken(token)
	}
	return client
}
