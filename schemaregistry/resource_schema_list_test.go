package schemaregistry

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	uuid "github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/riferrei/srclient"
)

func TestFilterSubjects(t *testing.T) {
	in := []string{"b-orders", "a-users", "b-items", "c"}
	if got, want := filterSubjects(in, "b-"), []string{"b-items", "b-orders"}; !slices.Equal(got, want) {
		t.Errorf("prefix b-: got %v, want %v", got, want)
	}
	if got, want := filterSubjects(in, ""), []string{"a-users", "b-items", "b-orders", "c"}; !slices.Equal(got, want) {
		t.Errorf("no prefix: got %v, want %v", got, want)
	}
	if in[0] != "b-orders" {
		t.Error("input slice was modified")
	}
}

func TestAccSchemaListResource_basic(t *testing.T) {
	client := testAccClient(t)

	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("list-%s-", u)
	subject := prefix + "value"

	if _, err := client.CreateSchema(subject, strings.ReplaceAll(fixtureAvro1, "\\", ""), srclient.Avro); err != nil {
		t.Fatalf("could not create schema for subject %s: %s", subject, err)
	}
	t.Cleanup(func() {
		_ = client.DeleteSubject(subject, false)
		_ = client.DeleteSubject(subject, true)
	})

	identity := map[string]knownvalue.Check{"subject": knownvalue.StringExact(subject)}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		Steps: []resource.TestStep{
			{
				Query: true,
				Config: fmt.Sprintf(`
provider "schemaregistry" {}

list "schemaregistry_schema" "test" {
  provider         = schemaregistry
  include_resource = true

  config {
    prefix = %[1]q
  }
}
`, prefix),
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectLength("schemaregistry_schema.test", 1),
					querycheck.ExpectIdentity("schemaregistry_schema.test", identity),
					querycheck.ExpectResourceKnownValues(
						"schemaregistry_schema.test",
						queryfilter.ByResourceIdentity(identity),
						[]querycheck.KnownValueCheck{
							{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact(subject)},
							{Path: tfjsonpath.New("subject"), KnownValue: knownvalue.StringExact(subject)},
							{Path: tfjsonpath.New("version"), KnownValue: knownvalue.Int64Exact(1)},
						},
					),
				},
			},
		},
	})
}
