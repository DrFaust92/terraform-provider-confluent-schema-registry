resource "schemaregistry_schema" "example" {
  subject = "example"
  schema = jsonencode({
    type = "record"
    name = "Example"
    fields = [
      {
        name = "id"
        type = "string"
      }
    ]
  })
}

resource "schemaregistry_schema" "protobuf_example" {
  subject     = "example-protobuf"
  schema_type = "PROTOBUF"
  schema      = <<-EOT
    syntax = "proto3";
    package example;

    message UserAdded {
      string first_name = 1;
    }
  EOT
}
