---
layout: "schemaregistry"
page_title: "Schema Registry: schemaregistry_schema"
sidebar_current: "docs-schemaregistry-resource-schema"
description: |-
  Provides a Schema Registry Schema
---

# schemaregistry\_schema

Provides a Schema Registry Schema resource.

## Example Usage

```hcl
resource "schemaregistry_schema" "schema" {
  subject = "example"
  schema  = "example"
}
```

## Argument Reference

The following arguments are supported:

* `subject` - (Required) The subject related to the schema.
* `schema` - (Required) The schema string.
* `reference` - (Optional) The referenced schema list.

## Attributes Reference

* `schema_id` - The ID of the schema.
* `version` - The schema version.

## Import

Schemas can be imported using their `subject` ID, e.g.

```sh
terraform import schemaregistry_schema.example subject
```
