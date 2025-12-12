---
layout: "schema registry"
page_title: "Provider: Kafka Schema Registry"
sidebar_current: "docs-schema registry-index"
description: |-
  The Kafka Schema Registry provider to interact with schemas
---

# Schema Registry Provider

The Schema Registry provider allows you to manage schema resources.

Use the navigation to the left to read about the available resources.

## Example Usage

### Basic Authentication

```hcl
# Configure the Schema Registry Provider with basic authentication
provider "schemaregistry" {
  schema_registry_url = "https://my.cool.registry"
  username            = "GobBluthe"
  password            = "idoillusions"
}

resource "schemaregistry_schema" "schema" {
  subject = "example"
  schema  = "example"
}
```

### OAuth2 Client Credentials

```hcl
# Configure the Schema Registry Provider with OAuth2
provider "schemaregistry" {
  schema_registry_url = "https://my.cool.registry"
  oauth2_token_url = "https://auth-server.com/token"
  oauth2_client_id = "client_id"
  oauth2_client_secret = "client_secret"
}

resource "schemaregistry_schema" "schema" {
  subject = "example"
  schema  = "example"
}
```

### Static Bearer Token

```hcl
# Configure the Schema Registry Provider with a static bearer token
provider "schemaregistry" {
  schema_registry_url = "https://my.cool.registry"
  bearer_token        = "your-static-bearer-token"
}

resource "schemaregistry_schema" "schema" {
  subject = "example"
  schema  = "example"
}
```

## Argument Reference

The following arguments are supported in the `provider` block:

* `schema_registry_url` - (Optional) The URL of the schema registry instance.
  You can also set this via the `SCHEMA_REGISTRY_URL` environment variable.

### Authentication Methods

**Note:** Only one authentication method can be configured at a time. Choose one of the following:

#### Basic Authentication (Provider Block)

* `username` - (Optional) The username used to authenticate against the schema registry instance.
  You can also set this via the `SCHEMA_REGISTRY_USERNAME` environment variable.

* `password` - (Optional) The password used to authenticate against the schema registry instance.
  You can also set this via the `SCHEMA_REGISTRY_PASSWORD` environment variable.

#### Bearer Token Authentication

* `bearer_token` - (Optional) A static bearer token used to authenticate against the schema registry instance.
  You can also set this via the `SCHEMA_REGISTRY_BEARER_TOKEN` environment variable.

#### OAuth2 Client Credentials

* `oauth2_token_url` - (Optional) OAuth2 token endpoint URL.
   You can also set this via the `SCHEMA_REGISTRY_OAUTH2_TOKEN_URL` environment variable.
* `oauth2_client_id` - (Optional) OAuth2 client ID.
   You can also set this via the `SCHEMA_REGISTRY_OAUTH2_CLIENT_ID` environment variable.
* `oauth2_client_secret` - (Optional) OAuth2 client secret
   You can also set this via the `SCHEMA_REGISTRY_OAUTH2_CLIENT_SECRET` environment variable.
* `oauth2_scopes` - (Optional) List of OAuth2 scopes to request.


## Environment Variables

You can configure the provider using environment variables instead of explicit configuration:

### Basic Auth

```bash
export SCHEMA_REGISTRY_URL="https://my.cool.registry"
export SCHEMA_REGISTRY_USERNAME="GobBluthe"
export SCHEMA_REGISTRY_PASSWORD="idoillusions"
```

### OAuth2 Auth

```bash
export SCHEMA_REGISTRY_URL="https://my.cool.registry"
export SCHEMA_REGISTRY_OAUTH2_TOKEN_URL="https://auth.example.com/oauth2/token"
export SCHEMA_REGISTRY_OAUTH2_CLIENT_ID="your-client-id"
export SCHEMA_REGISTRY_OAUTH2_CLIENT_SECRET="your-client-secret"
```

### Bearer Token Auth

```bash
export SCHEMA_REGISTRY_URL="https://my.cool.registry"
export SCHEMA_REGISTRY_BEARER_TOKEN="your-static-bearer-token"
```
