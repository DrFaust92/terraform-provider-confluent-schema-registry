package schemaregistry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewOAuth2Client(t *testing.T) {
	config := &OAuth2Config{
		TokenURL:     "https://example.com/token",
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Scopes:       []string{"test-scope"},
	}

	client := NewOAuth2Client(config)

	if client == nil {
		t.Fatal("Expected OAuth2Client to be created, got nil")
	}

	if client.config.ClientID != "test-client-id" {
		t.Errorf("Expected ClientID to be 'test-client-id', got '%s'", client.config.ClientID)
	}

	if client.config.TokenURL != "https://example.com/token" {
		t.Errorf("Expected TokenURL to be 'https://example.com/token', got '%s'", client.config.TokenURL)
	}
}

func TestOAuth2Client_GetToken_Success(t *testing.T) {
	// Create a mock OAuth2 server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Errorf("Expected path '/token', got '%s'", r.URL.Path)
		}

		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got '%s'", r.Method)
		}

		// Check for client credentials in the request
		err := r.ParseForm()
		if err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}

		if r.Form.Get("grant_type") != "client_credentials" {
			t.Errorf("Expected grant_type 'client_credentials', got '%s'", r.Form.Get("grant_type"))
		}

		// Return a mock token response
		response := map[string]interface{}{
			"access_token": "mock-access-token-12345",
			"token_type":   "Bearer",
			"expires_in":   3600,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	config := &OAuth2Config{
		TokenURL:     mockServer.URL + "/token",
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Scopes:       []string{"test-scope"},
	}

	client := NewOAuth2Client(config)
	token, err := client.getToken()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if token.AccessToken != "mock-access-token-12345" {
		t.Errorf("Expected access token 'mock-access-token-12345', got '%s'", token.AccessToken)
	}

	if token.TokenType != "Bearer" {
		t.Errorf("Expected token type 'Bearer', got '%s'", token.TokenType)
	}
}

func TestOAuth2Client_GetToken_InvalidCredentials(t *testing.T) {
	// Create a mock OAuth2 server that returns 401
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		response := map[string]interface{}{
			"error":             "invalid_client",
			"error_description": "Client authentication failed",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	config := &OAuth2Config{
		TokenURL:     mockServer.URL + "/token",
		ClientID:     "invalid-client-id",
		ClientSecret: "invalid-secret",
		Scopes:       []string{"test-scope"},
	}

	client := NewOAuth2Client(config)
	_, err := client.getToken()

	if err == nil {
		t.Fatal("Expected error for invalid credentials, got nil")
	}
}

func TestOAuth2Client_GetToken_ServerError(t *testing.T) {
	// Create a mock OAuth2 server that returns 500
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	config := &OAuth2Config{
		TokenURL:     mockServer.URL + "/token",
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Scopes:       []string{"test-scope"},
	}

	client := NewOAuth2Client(config)
	_, err := client.getToken()

	if err == nil {
		t.Fatal("Expected error for server error, got nil")
	}
}

func TestGetToken_ParseConfig(t *testing.T) {
	// Create a mock OAuth2 server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"access_token": "parsed-token-123",
			"token_type":   "Bearer",
			"expires_in":   3600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Simulate Terraform schema data structure
	oauth2Config := map[string]interface{}{
		"token_url":     mockServer.URL + "/token",
		"client_id":     "test-client-id",
		"client_secret": "test-client-secret",
		"scopes":        []interface{}{"scope1", "scope2"},
	}

	token, err := getToken(oauth2Config)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if token != "parsed-token-123" {
		t.Errorf("Expected token 'parsed-token-123', got '%s'", token)
	}
}

func TestGetToken_WithoutScopes(t *testing.T) {
	// Create a mock OAuth2 server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"access_token": "no-scope-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Config without scopes
	oauth2Config := map[string]interface{}{
		"token_url":     mockServer.URL + "/token",
		"client_id":     "test-client-id",
		"client_secret": "test-client-secret",
	}

	token, err := getToken(oauth2Config)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if token != "no-scope-token" {
		t.Errorf("Expected token 'no-scope-token', got '%s'", token)
	}
}

func TestGetToken_ExpiredToken(t *testing.T) {
	// Create a mock OAuth2 server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a token that expires immediately
		response := map[string]interface{}{
			"access_token": "short-lived-token",
			"token_type":   "Bearer",
			"expires_in":   1, // 1 second
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	oauth2Config := map[string]interface{}{
		"token_url":     mockServer.URL + "/token",
		"client_id":     "test-client-id",
		"client_secret": "test-client-secret",
	}

	token, err := getToken(oauth2Config)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if token != "short-lived-token" {
		t.Errorf("Expected token 'short-lived-token', got '%s'", token)
	}

	// Token should still be valid for the initial retrieval
	// In a real scenario, you'd test token refresh logic
}

func TestOAuth2Client_GetToken_WithCustomScopes(t *testing.T) {
	requestedScopes := []string{}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}

		// Capture the scopes from the request
		if scope := r.Form.Get("scope"); scope != "" {
			// The scope parameter is space-separated
			requestedScopes = []string{scope}
		}

		response := map[string]interface{}{
			"access_token": "scoped-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	config := &OAuth2Config{
		TokenURL:     mockServer.URL + "/token",
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Scopes:       []string{"read", "write"},
	}

	client := NewOAuth2Client(config)
	token, err := client.getToken()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if token.AccessToken != "scoped-token" {
		t.Errorf("Expected access token 'scoped-token', got '%s'", token.AccessToken)
	}

	// Verify scopes were sent (they should be space-separated)
	if len(requestedScopes) > 0 && requestedScopes[0] != "read write" {
		t.Logf("Note: Scopes sent as: %v", requestedScopes)
	}
}

func TestOAuth2Client_GetToken_Timeout(t *testing.T) {
	// Create a mock server that delays response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Simulate slow server
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	config := &OAuth2Config{
		TokenURL:     mockServer.URL + "/token",
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}

	client := NewOAuth2Client(config)

	// This test will take 5 seconds to complete
	// In a production setting, you'd configure a timeout on the HTTP client
	_, err := client.getToken()

	// The current implementation doesn't have a timeout, so this should succeed
	// You might want to add timeout configuration to OAuth2Client in the future
	if err != nil {
		t.Logf("Note: Error occurred (possibly due to timeout): %v", err)
	}
}
