package schemaregistry

import (
	"context"
	"os"
	"testing"

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
