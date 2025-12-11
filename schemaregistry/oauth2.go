package schemaregistry

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type OAuth2Config struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

type OAuth2Client struct {
	config     *OAuth2Config
	httpClient *http.Client
}

func NewOAuth2Client(config *OAuth2Config) *OAuth2Client {
	return &OAuth2Client{
		config:     config,
		httpClient: &http.Client{},
	}
}

func (c *OAuth2Client) getToken() (*oauth2.Token, error) {
	config := &clientcredentials.Config{
		ClientID:     c.config.ClientID,
		ClientSecret: c.config.ClientSecret,
		TokenURL:     c.config.TokenURL,
		Scopes:       c.config.Scopes,
	}

	return config.Token(context.Background())
}
