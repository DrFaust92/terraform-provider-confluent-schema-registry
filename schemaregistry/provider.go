package schemaregistry

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/riferrei/srclient"
)

var _ provider.Provider = &schemaRegistryProvider{}

type schemaRegistryProvider struct {
	version string
}

// New returns a constructor for the framework provider.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &schemaRegistryProvider{version: version}
	}
}

func (p *schemaRegistryProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "schemaregistry"
	resp.Version = p.version
}

func (p *schemaRegistryProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"schema_registry_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The Schema Registry URL. May also be set via the `SCHEMA_REGISTRY_URL` environment variable.",
			},
			"username": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Username for basic authentication. May also be set via `SCHEMA_REGISTRY_USERNAME`.",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Password for basic authentication. May also be set via `SCHEMA_REGISTRY_PASSWORD`.",
			},
			"bearer_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Static bearer token for authentication. May also be set via `SCHEMA_REGISTRY_BEARER_TOKEN`.",
			},
			"oauth2_token_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "OAuth2 token endpoint URL. May also be set via `SCHEMA_REGISTRY_OAUTH2_TOKEN_URL`.",
			},
			"oauth2_client_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "OAuth2 client ID. May also be set via `SCHEMA_REGISTRY_OAUTH2_CLIENT_ID`.",
			},
			"oauth2_client_secret": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "OAuth2 client secret. May also be set via `SCHEMA_REGISTRY_OAUTH2_CLIENT_SECRET`.",
			},
			"oauth2_scopes": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "OAuth2 scopes to request.",
			},
		},
	}
}

type schemaRegistryProviderModel struct {
	URL                types.String `tfsdk:"schema_registry_url"`
	Username           types.String `tfsdk:"username"`
	Password           types.String `tfsdk:"password"`
	BearerToken        types.String `tfsdk:"bearer_token"`
	OAuth2TokenURL     types.String `tfsdk:"oauth2_token_url"`
	OAuth2ClientID     types.String `tfsdk:"oauth2_client_id"`
	OAuth2ClientSecret types.String `tfsdk:"oauth2_client_secret"`
	OAuth2Scopes       types.List   `tfsdk:"oauth2_scopes"`
}

func (p *schemaRegistryProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config schemaRegistryProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := stringOrEnv(config.URL, "SCHEMA_REGISTRY_URL")
	username := stringOrEnv(config.Username, "SCHEMA_REGISTRY_USERNAME")
	password := stringOrEnv(config.Password, "SCHEMA_REGISTRY_PASSWORD")
	bearerToken := stringOrEnv(config.BearerToken, "SCHEMA_REGISTRY_BEARER_TOKEN")
	oauth2TokenURL := stringOrEnv(config.OAuth2TokenURL, "SCHEMA_REGISTRY_OAUTH2_TOKEN_URL")
	oauth2ClientID := stringOrEnv(config.OAuth2ClientID, "SCHEMA_REGISTRY_OAUTH2_CLIENT_ID")
	oauth2ClientSecret := stringOrEnv(config.OAuth2ClientSecret, "SCHEMA_REGISTRY_OAUTH2_CLIENT_SECRET")

	if url == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("schema_registry_url"),
			"Missing Schema Registry URL",
			"schema_registry_url must be set in the configuration or via the SCHEMA_REGISTRY_URL environment variable.",
		)
		return
	}

	hasBasicAuth := username != "" && password != ""
	hasOAuth2 := oauth2TokenURL != "" && oauth2ClientID != "" && oauth2ClientSecret != ""

	authMethods := 0
	if hasBasicAuth {
		authMethods++
	}
	if bearerToken != "" {
		authMethods++
	}
	if hasOAuth2 {
		authMethods++
	}
	if authMethods > 1 {
		resp.Diagnostics.AddError(
			"Conflicting authentication methods",
			"only one of username/password, bearer_token, or oauth2 can be set",
		)
		return
	}

	client := srclient.CreateSchemaRegistryClient(url)
	switch {
	case hasBasicAuth:
		client.SetCredentials(username, password)
	case bearerToken != "":
		client.SetBearerToken(bearerToken)
	case hasOAuth2:
		var scopes []string
		if !config.OAuth2Scopes.IsNull() && !config.OAuth2Scopes.IsUnknown() {
			resp.Diagnostics.Append(config.OAuth2Scopes.ElementsAs(ctx, &scopes, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
		token, err := getToken(oauth2TokenURL, oauth2ClientID, oauth2ClientSecret, scopes)
		if err != nil {
			resp.Diagnostics.AddError("Failed to obtain OAuth2 token", err.Error())
			return
		}
		client.SetBearerToken(token)
	}

	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *schemaRegistryProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newSchemaResource,
	}
}

func (p *schemaRegistryProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newSchemaDataSource,
	}
}

// stringOrEnv returns the configured value when known and non-null, otherwise
// the named environment variable.
func stringOrEnv(v types.String, envKey string) string {
	if !v.IsNull() && !v.IsUnknown() {
		return v.ValueString()
	}
	return os.Getenv(envKey)
}

func getToken(tokenURL, clientID, clientSecret string, scopes []string) (string, error) {
	oauth2Client := NewOAuth2Client(&OAuth2Config{
		TokenURL:     tokenURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       scopes,
	})
	token, err := oauth2Client.getToken()
	if err != nil {
		return "", err
	}
	return token.AccessToken, nil
}
