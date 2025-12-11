# OAuth2 Authentication Support Implementation Plan

## Overview
Add OAuth2/OIDC authentication support to the Terraform provider for Confluent Schema Registry, enabling users to authenticate using OAuth2 client credentials flow in addition to the existing basic authentication.

## Background

### Current State
- Provider currently supports only basic authentication (username/password)
- Uses `github.com/riferrei/srclient` v0.7.3 which has `SetBearerToken()` method
- Authentication is configured in `schemaregistry/provider.go`

### Target State
- Support OAuth2 client credentials flow
- Maintain backward compatibility with basic auth
- Support both static bearer tokens and dynamic OAuth2 token acquisition
- Handle token refresh automatically

### References
- Confluent Schema Registry OAuth Support: https://docs.confluent.io/platform/current/schema-registry/security/oauth-schema-registry.html
- Confluent Cloud OAuth Client Configuration: https://docs.confluent.io/cloud/current/security/authenticate/workload-identities/identity-providers/oauth/configure-clients-oauth.html
- srclient library: https://pkg.go.dev/github.com/riferrei/srclient

## Implementation Plan

### 1. Provider Schema Updates (`schemaregistry/provider.go`)

#### 1.1 Add OAuth2 Configuration Fields
Add the following optional fields to the provider schema:

```go
// OAuth2 Configuration
"oauth2": {
    Type:     schema.TypeList,
    Optional: true,
    MaxItems: 1,
    Elem: &schema.Resource{
        Schema: map[string]*schema.Schema{
            "token_url": {
                Type:        schema.TypeString,
                Required:    true,
                Description: "OAuth2 token endpoint URL",
                DefaultFunc: schema.EnvDefaultFunc("SCHEMA_REGISTRY_OAUTH2_TOKEN_URL", nil),
            },
            "client_id": {
                Type:        schema.TypeString,
                Required:    true,
                Description: "OAuth2 client ID",
                DefaultFunc: schema.EnvDefaultFunc("SCHEMA_REGISTRY_OAUTH2_CLIENT_ID", nil),
            },
            "client_secret": {
                Type:        schema.TypeString,
                Required:    true,
                Sensitive:   true,
                Description: "OAuth2 client secret",
                DefaultFunc: schema.EnvDefaultFunc("SCHEMA_REGISTRY_OAUTH2_CLIENT_SECRET", nil),
            },
            "scopes": {
                Type:        schema.TypeList,
                Optional:    true,
                Description: "OAuth2 scopes to request",
                Elem:        &schema.Schema{Type: schema.TypeString},
            },
        },
    },
},
// Static bearer token option (simpler alternative)
"bearer_token": {
    Type:        schema.TypeString,
    Optional:    true,
    Sensitive:   true,
    Description: "Static bearer token for authentication",
    DefaultFunc: schema.EnvDefaultFunc("SCHEMA_REGISTRY_BEARER_TOKEN", nil),
},
```

#### 1.2 Update Validation Logic
- Ensure mutual exclusivity: user can provide either (username+password) OR oauth2 config OR bearer_token
- Add validation in `providerConfigure` to check for conflicting auth methods

### 2. Create OAuth2 Client Module (`schemaregistry/oauth2_client.go`)

#### 2.1 Define OAuth2 Client Structure
```go
type OAuth2Config struct {
    TokenURL     string
    ClientID     string
    ClientSecret string
    Scopes       []string
}

type OAuth2Client struct {
    config      *OAuth2Config
    httpClient  *http.Client
    token       *oauth2.Token
    mutex       sync.RWMutex
}
```

#### 2.2 Implement Key Methods
- `NewOAuth2Client(config *OAuth2Config) (*OAuth2Client, error)`: Constructor
- `GetToken() (string, error)`: Get current valid token (fetch new if expired)
- `fetchToken() (*oauth2.Token, error)`: Fetch new token from OAuth2 server
- `isTokenValid() bool`: Check if current token is valid and not expired

#### 2.3 Token Lifecycle Management
- Implement token caching to avoid unnecessary requests
- Add token expiry checking with buffer (e.g., refresh 5 minutes before expiry)
- Handle token refresh on 401 responses (implement retry logic)

### 3. Create Custom HTTP Transport (`schemaregistry/oauth2_transport.go`)

#### 3.1 Bearer Token Transport
Create a custom `http.RoundTripper` that:
- Wraps the default HTTP transport
- Automatically adds Bearer token to all requests
- Handles token refresh on 401 errors
- Integrates with OAuth2Client for token management

```go
type OAuth2Transport struct {
    Base        http.RoundTripper
    oauth2Client *OAuth2Client
}

func (t *OAuth2Transport) RoundTrip(req *http.Request) (*http.Response, error) {
    // Clone request
    // Add Authorization header with bearer token
    // Execute request
    // Handle 401 by refreshing token and retrying once
}
```

### 4. Update Provider Configuration (`schemaregistry/provider.go`)

#### 4.1 Modify `providerConfigure` Function
Update the configuration logic to:

1. **Validate authentication method:**
   - Check for conflicting auth configurations
   - Ensure at least one auth method is provided

2. **Initialize Schema Registry client based on auth type:**

   **For Basic Auth (existing):**
   ```go
   if username != "" && password != "" {
       client.SetCredentials(username, password)
   }
   ```

   **For Static Bearer Token:**
   ```go
   if bearerToken != "" {
       client.SetBearerToken(bearerToken)
   }
   ```

   **For OAuth2:**
   ```go
   if oauth2Config != nil {
       oauth2Client, err := NewOAuth2Client(oauth2Config)
       if err != nil {
           return nil, diag.FromErr(err)
       }

       // Get initial token to validate configuration
       token, err := oauth2Client.GetToken()
       if err != nil {
           return nil, diag.FromErr(err)
       }

       client.SetBearerToken(token)

       // Store oauth2Client in metadata for token refresh
       // (may need to create wrapper struct)
   }
   ```

#### 4.2 Consider Client Wrapper
Since srclient doesn't have built-in token refresh, consider creating a wrapper:

```go
type ProviderClient struct {
    SchemaRegistryClient *srclient.SchemaRegistryClient
    OAuth2Client         *OAuth2Client
}
```

This wrapper can:
- Expose srclient methods
- Handle automatic token refresh before requests
- Be passed as `meta interface{}` to resource/data source functions

### 5. Update Dependencies (`go.mod`)

Add required OAuth2 library:
```bash
go get golang.org/x/oauth2
```

### 6. Documentation Updates

#### 6.1 Update README.md
Add OAuth2 configuration examples:

**Using OAuth2 Client Credentials:**
```hcl
provider "schemaregistry" {
  schema_registry_url = "https://xxxxx.confluent.cloud"

  oauth2 {
    token_url     = "https://auth.example.com/oauth2/token"
    client_id     = "your-client-id"
    client_secret = "your-client-secret"
    scopes        = ["schema-registry"]
  }
}
```

**Using Static Bearer Token:**
```hcl
provider "schemaregistry" {
  schema_registry_url = "https://xxxxx.confluent.cloud"
  bearer_token        = "your-bearer-token"
}
```

**Using Environment Variables:**
```bash
export SCHEMA_REGISTRY_URL="https://xxxxx.confluent.cloud"
export SCHEMA_REGISTRY_OAUTH2_TOKEN_URL="https://auth.example.com/oauth2/token"
export SCHEMA_REGISTRY_OAUTH2_CLIENT_ID="your-client-id"
export SCHEMA_REGISTRY_OAUTH2_CLIENT_SECRET="your-client-secret"
export SCHEMA_REGISTRY_BEARER_TOKEN="your-token"  # Alternative to OAuth2
```

#### 6.2 Create Migration Guide
Document migration path for users moving from basic auth to OAuth2

### 7. Testing Strategy

#### 7.1 Unit Tests

**File: `schemaregistry/oauth2_client_test.go`**
- Test OAuth2 token fetching
- Test token caching
- Test token expiry and refresh
- Test error handling (invalid credentials, unreachable token endpoint)
- Mock HTTP responses for token endpoint

**File: `schemaregistry/oauth2_transport_test.go`**
- Test Bearer token injection
- Test 401 retry logic
- Test request cloning

**File: `schemaregistry/provider_test.go`**
- Test provider configuration validation
- Test mutual exclusivity of auth methods
- Test OAuth2 configuration parsing
- Test environment variable defaults

#### 7.2 Integration Tests
- Test against actual OAuth2-enabled Schema Registry (if available)
- Test against mock OAuth2 server
- Test full resource lifecycle (create, read, update, delete) with OAuth2

#### 7.3 Acceptance Tests
Update existing acceptance tests to support OAuth2 authentication:
- Add OAuth2 test configuration
- Ensure backward compatibility with basic auth tests

### 8. Error Handling & Edge Cases

#### 8.1 Error Scenarios to Handle
- OAuth2 token endpoint unreachable
- Invalid OAuth2 credentials
- Token expired and refresh failed
- Network timeout during token fetch
- Malformed token response
- Schema Registry returns 401 despite valid token (permission issue)

#### 8.2 Error Messages
Provide clear, actionable error messages:
- "OAuth2 authentication failed: unable to fetch token from {token_url}: {error}"
- "Bearer token authentication failed: received 401 from Schema Registry"
- "Invalid provider configuration: cannot use both basic auth and OAuth2"

### 9. Performance Considerations

#### 9.1 Token Caching
- Cache tokens in memory to avoid repeated OAuth2 calls
- Implement thread-safe token access with mutex
- Refresh token proactively before expiry (e.g., 5 minutes buffer)

#### 9.2 HTTP Client Reuse
- Reuse HTTP client instances
- Configure reasonable timeouts (e.g., 30 seconds for token fetch)
- Consider connection pooling

### 10. Security Considerations

#### 10.1 Sensitive Data Handling
- Mark `client_secret` and `bearer_token` as `Sensitive: true` in schema
- Ensure tokens are not logged
- Clear tokens from memory when provider is destroyed (if applicable)

#### 10.2 Token Storage
- Store tokens only in memory, never persist to disk
- Consider security implications of token in Terraform state (document this)

### 11. Backward Compatibility

#### 11.1 Ensure No Breaking Changes
- Keep existing `username` and `password` fields functional
- Default to basic auth if OAuth2 not configured
- Maintain existing error messages and behavior for basic auth

#### 11.2 Deprecation Path (Future)
- No deprecation planned for basic auth
- Both auth methods should coexist

## Implementation Order

### Phase 1: Core OAuth2 Support
1. Add OAuth2 dependencies to `go.mod`
2. Create `oauth2_client.go` with token management
3. Update provider schema in `provider.go`
4. Modify `providerConfigure` to support OAuth2
5. Add unit tests

### Phase 2: Bearer Token Support
1. Add static bearer token field to schema
2. Update `providerConfigure` for bearer token
3. Add validation for auth method exclusivity
4. Add unit tests

### Phase 3: Advanced Features
1. Create OAuth2 transport for automatic token refresh
2. Implement retry logic for 401 errors
3. Add integration tests

### Phase 4: Documentation & Testing
1. Update README.md with examples
2. Add comprehensive acceptance tests
3. Create migration guide
4. Update examples directory (if exists)

## Testing Checklist

- [ ] Unit tests for OAuth2 client
- [ ] Unit tests for token refresh
- [ ] Unit tests for provider configuration validation
- [ ] Integration test with mock OAuth2 server
- [ ] Acceptance test with OAuth2 auth
- [ ] Backward compatibility test with basic auth
- [ ] Test with invalid OAuth2 credentials
- [ ] Test with expired tokens
- [ ] Test token refresh on 401

## Success Criteria

1. Users can authenticate to Schema Registry using OAuth2 client credentials
2. Users can authenticate using static bearer tokens
3. Existing basic auth functionality remains unchanged
4. Token refresh happens automatically when needed
5. Clear error messages for OAuth2 configuration issues
6. All tests pass (unit, integration, acceptance)
7. Documentation is comprehensive and includes examples
8. No breaking changes to existing provider API

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| srclient library doesn't support token refresh | Medium | Create custom wrapper or HTTP transport to inject fresh tokens |
| OAuth2 token endpoint is slow | Low | Implement token caching and background refresh |
| Breaking changes to existing users | High | Maintain backward compatibility, extensive testing |
| State file contains sensitive OAuth2 tokens | Medium | Document security implications, consider using externally managed tokens |

## Future Enhancements

1. Support for other OAuth2 flows (authorization code, device code)
2. Support for mTLS authentication
3. Support for AWS IAM authentication (if using AWS MSK)
4. Configurable token refresh buffer time
5. Metrics/logging for authentication events
