package schemaregistry

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/riferrei/srclient"
)

// Provider -
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"schema_registry_url": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("SCHEMA_REGISTRY_URL", nil),
			},
			"username": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("SCHEMA_REGISTRY_USERNAME", nil),
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("SCHEMA_REGISTRY_PASSWORD", nil),
			},
			"bearer_token": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Static bearer token for authentication",
				DefaultFunc: schema.EnvDefaultFunc("SCHEMA_REGISTRY_BEARER_TOKEN", nil),
			},
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
		},
		ResourcesMap: map[string]*schema.Resource{
			"schemaregistry_schema": resourceSchema(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"schemaregistry_schema": dataSourceSchema(),
		},
		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	url := d.Get("schema_registry_url").(string)
	username := d.Get("username").(string)
	password := d.Get("password").(string)
	bearerToken := d.Get("bearer_token").(string)
	oauth2List := d.Get("oauth2").([]interface{})

	// Validate that only one auth method is used
	authMethods := 0
	if username != "" && password != "" {
		authMethods++
	}
	if bearerToken != "" {
		authMethods++
	}
	if len(oauth2List) > 0 {
		authMethods++
	}
	if authMethods > 1 {
		return nil, diag.FromErr(errors.New("only one of username/password, bearer_token, or oauth2 can be set"))
	}
	// Warning or errors can be collected in a slice type
	var diags diag.Diagnostics

	if url != "" {
		client := srclient.CreateSchemaRegistryClient(url)

		if (username != "") && (password != "") {
			client.SetCredentials(username, password)
		}
		if bearerToken != "" {
			client.SetBearerToken(bearerToken)
		}
		if len(oauth2List) > 0 {
			oauth2Map := oauth2List[0].(map[string]interface{})

			token, err := getToken(oauth2Map)
			if err != nil {
				return nil, diag.FromErr(err)
			}

			client.SetBearerToken(token)
		}
		return client, diags
	}

	return nil, diag.FromErr(errors.New("invalid credential parameters"))
}

func getToken(oauth2Config map[string]interface{}) (string, error) {
	// Parse OAuth2 configuration
	tokenURL := oauth2Config["token_url"].(string)
	clientID := oauth2Config["client_id"].(string)
	clientSecret := oauth2Config["client_secret"].(string)

	// Parse scopes (optional)
	var scopes []string
	if scopesInterface, ok := oauth2Config["scopes"]; ok && scopesInterface != nil {
		scopesList := scopesInterface.([]interface{})
		scopes = make([]string, len(scopesList))
		for i, scope := range scopesList {
			scopes[i] = scope.(string)
		}
	}

	// Create OAuth2 client and get token
	config := &OAuth2Config{
		TokenURL:     tokenURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       scopes,
	}

	oauth2Client := NewOAuth2Client(config)
	token, err := oauth2Client.getToken()
	if err != nil {
		return "", err
	}

	return token.AccessToken, nil
}
