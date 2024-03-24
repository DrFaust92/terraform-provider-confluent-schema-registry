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

```hcl
# Configure the Schema Registry Provider
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

## Argument Reference

The following arguments are supported in the `provider` block:

* `schema_registry_url` - (Optional) The URL of the schema registry instance.
  You can also set this via the `SCHEMA_REGISTRY_URL` environment variable.

* `username` - (Optional) The username used to authentiacte against the schema registry instance.
  You can also set this via the `SCHEMA_REGISTRY_USERNAME` environment variable.

* `password` - (Optional) The password used to authentiacte against the schema registry instance.
  You can also set this via the `SCHEMA_REGISTRY_PASSWORD` environment variable.
