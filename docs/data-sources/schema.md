---
layout: "schemaregistry"
page_title: "Schema Registry: schemaregistry_schema"
sidebar_current: "docs-schemaregistry-data-schema"
description: |-
  Provides a data source for Schema Registry Schema
---

# schemaregistry\_schema

Provides a Schema Registry Schema resource.

## Example Usage

```hcl
data "schemaregistry_schema" "example" {
	subject = "example"
	version = 1
}
```

## Argument Reference

The following arguments are supported:

* `subject` - (Required) The subject related to the schema.
* `version` - (Optional) The schema version to fetch.

## Attributes Reference

* `schema_id` - The ID of the schema.
* `references` - The schema references.
* `schema` - The schema string.
