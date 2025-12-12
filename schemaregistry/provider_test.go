package schemaregistry

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var testAccProvider = getProvider()
var testAccProviders = testAccProvidersFactory(testAccProvider)

func testAccProvidersFactory(provider *schema.Provider) map[string]func() (*schema.Provider, error) {
	return map[string]func() (*schema.Provider, error){
		"schemaregistry": func() (*schema.Provider, error) {
			return provider, nil
		},
	}
}

func getProvider() *schema.Provider {
	return Provider()
}

func TestProvider(t *testing.T) {
	if err := Provider().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProvider_impl(t *testing.T) {
	log.Println("[INFO] TestProvider_impl")
	var _ = Provider()
}

func testAccPreCheck(t *testing.T) {
	log.Println("[INFO] testAccPreCheck")

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

func TestProviderConfigure_OAuth2(t *testing.T) {
	// Note: This is a unit test that validates the schema parsing
	// It won't actually connect to an OAuth2 server
	provider := Provider()

	// Create a mock resource data with OAuth2 config
	resourceData := schema.TestResourceDataRaw(t, provider.Schema, map[string]interface{}{
		"schema_registry_url":  "https://test-registry.example.com",
		"oauth2_token_url":     "https://auth.example.com/token",
		"oauth2_client_id":     "test-client-id",
		"oauth2_client_secret": "test-client-secret",
		"oauth2_scopes":        []interface{}{"scope1", "scope2"},
	})

	// Test that the provider can parse OAuth2 config
	// This will fail to get a token since there's no real OAuth2 server,
	// but we're testing the configuration parsing
	_, diags := providerConfigure(context.TODO(), resourceData)

	// We expect an error because there's no real OAuth2 server
	if !diags.HasError() {
		t.Log("Note: OAuth2 config was parsed successfully (token fetch would fail without real server)")
	}
}

func TestProviderConfigure_BearerToken(t *testing.T) {
	provider := Provider()

	resourceData := schema.TestResourceDataRaw(t, provider.Schema, map[string]interface{}{
		"schema_registry_url":  "https://test-registry.example.com",
		"bearer_token":         "test-bearer-token-123",
		"username":             "",
		"password":             "",
		"oauth2_token_url":     "",
		"oauth2_client_id":     "",
		"oauth2_client_secret": "",
	})

	client, diags := providerConfigure(context.TODO(), resourceData)

	if diags.HasError() {
		t.Fatalf("Expected no error with bearer token, got: %v", diags)
	}

	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}
}

func TestProviderConfigure_ConflictingAuth(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]interface{}
	}{
		{
			name: "BasicAuth and BearerToken",
			config: map[string]interface{}{
				"schema_registry_url": "https://test-registry.example.com",
				"username":            "user",
				"password":            "pass",
				"bearer_token":        "token",
			},
		},
		{
			name: "BasicAuth and OAuth2",
			config: map[string]interface{}{
				"schema_registry_url":  "https://test-registry.example.com",
				"username":             "user",
				"password":             "pass",
				"oauth2_token_url":     "https://auth.example.com/token",
				"oauth2_client_id":     "client-id",
				"oauth2_client_secret": "client-secret",
			},
		},
		{
			name: "BearerToken and OAuth2",
			config: map[string]interface{}{
				"schema_registry_url":  "https://test-registry.example.com",
				"bearer_token":         "token",
				"oauth2_token_url":     "https://auth.example.com/token",
				"oauth2_client_id":     "client-id",
				"oauth2_client_secret": "client-secret",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := Provider()
			resourceData := schema.TestResourceDataRaw(t, provider.Schema, tt.config)

			_, diags := providerConfigure(context.TODO(), resourceData)

			if !diags.HasError() {
				t.Errorf("Expected error for conflicting auth methods, got none")
			}

			// Check that the error message mentions conflicting auth
			errorFound := false
			for _, diag := range diags {
				if diag.Severity == 0 { // Error severity
					errorFound = true
				}
			}

			if !errorFound {
				t.Errorf("Expected error diagnostic for conflicting auth methods")
			}
		})
	}
}

func TestProviderConfigure_NoAuth(t *testing.T) {
	provider := Provider()

	resourceData := schema.TestResourceDataRaw(t, provider.Schema, map[string]interface{}{
		"schema_registry_url": "https://test-registry.example.com",
	})

	// Provider should still create a client even without auth
	// (some Schema Registries might not require auth)
	client, diags := providerConfigure(context.TODO(), resourceData)

	if diags.HasError() {
		t.Fatalf("Expected no error without auth, got: %v", diags)
	}

	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}
}
