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
