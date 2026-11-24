package schemaregistry

import (
	"context"
	"fmt"
	"strings"
	"testing"

	uuid "github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const fixtureProtobuf1 = `syntax = "proto3";
package akc.test;

message UserAdded {
  string first_name = 1;
}
`

const fixtureProtobuf2 = `syntax = "proto3";
package akc.test;

message UserAdded {
  string first_name = 1;
  string last_name = 2;
}
`

func fixtureProtobufSchema(subject, schema string) string {
	return fmt.Sprintf(`
resource "schemaregistry_schema" "test" {
  subject     = %q
  schema_type = "PROTOBUF"
  schema      = <<-EOT
%sEOT
}

data "schemaregistry_schema" "test" {
  subject = schemaregistry_schema.test.subject
}
`, subject, schema)
}

func fixtureJSONSchema(subject string) string {
	return fmt.Sprintf(`
resource "schemaregistry_schema" "test" {
  subject     = %q
  schema_type = "JSON"
  schema = jsonencode({
    "$schema"  = "http://json-schema.org/draft-07/schema#"
    type       = "object"
    properties = { name = { type = "string" } }
  })
}
`, subject)
}

func testAccSubject(t *testing.T) string {
	t.Helper()
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("sub%s", u)
}

func TestAccResourceSchema_protobuf(t *testing.T) {
	resourceName := "schemaregistry_schema.test"
	subject := testAccSubject(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				// The post-apply refresh plan must be empty although the
				// registry stores a re-rendered form of the schema.
				Config: fixtureProtobufSchema(subject, fixtureProtobuf1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "schema_type", "PROTOBUF"),
					resource.TestCheckResourceAttr(resourceName, "version", "1"),
					resource.TestCheckResourceAttr(resourceName, "schema", fixtureProtobuf1),
					resource.TestCheckResourceAttr("data.schemaregistry_schema.test", "schema_type", "PROTOBUF"),
				),
			},
			{
				Config: fixtureProtobufSchema(subject, fixtureProtobuf2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "version", "2"),
					resource.TestCheckResourceAttr(resourceName, "schema", fixtureProtobuf2),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// Imported state holds the registry's canonical text.
				ImportStateVerifyIgnore: []string{"schema"},
			},
		},
	})
}

func TestAccResourceSchema_json(t *testing.T) {
	resourceName := "schemaregistry_schema.test"
	subject := testAccSubject(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: fixtureJSONSchema(subject),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "schema_type", "JSON"),
					resource.TestCheckResourceAttr(resourceName, "version", "1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccResourceSchema_upgradeFromV130 proves the change is non-breaking: an
// AVRO schema created by the last release (no schema_type attribute) plans
// with no changes after upgrading.
func TestAccResourceSchema_upgradeFromV130(t *testing.T) {
	resourceName := "schemaregistry_schema.test"
	subject := testAccSubject(t)
	config := fmt.Sprintf(fixtureCreateSchemaWithCompatibility, subject, fixtureAvro1, "BACKWARD")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		CheckDestroy: nil,
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"schemaregistry": {Source: "drfaust92/confluent-schema-registry", VersionConstraint: "1.3.0"},
				},
				Config: config,
			},
			{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr(resourceName, "schema_type", "AVRO"),
			},
		},
	})
}

func TestSchemaStringsEqual(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"json whitespace", `{"type":"string"}`, "{ \"type\" : \"string\" }", true},
		{"json different", `{"type":"string"}`, `{"type":"int"}`, false},
		{"json number representation kept", `{"a":1.0}`, `{"a":1}`, false},
		{"protobuf whitespace", fixtureProtobuf1, strings.ReplaceAll(fixtureProtobuf1, "\n", "\n\n  "), true},
		{"protobuf different", fixtureProtobuf1, fixtureProtobuf2, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := schemaStringsEqual(tc.a, tc.b); got != tc.want {
				t.Errorf("schemaStringsEqual = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCompatibilityValidatorCaseInsensitive(t *testing.T) {
	v := stringvalidator.OneOfCaseInsensitive(compatibilityLevels...)
	for _, value := range []string{"BACKWARD", "backward", "Full_Transitive", "NONE"} {
		resp := &validator.StringResponse{}
		v.ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue(value)}, resp)
		if resp.Diagnostics.HasError() {
			t.Errorf("%q rejected: %v", value, resp.Diagnostics)
		}
	}
	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue("SOMETIMES")}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("invalid compatibility level accepted")
	}
}

func TestResourceValidateConfigJSON(t *testing.T) {
	cases := []struct {
		name       string
		schemaType tftypes.Value
		schema     string
		wantErr    bool
	}{
		{"default avro, valid json", tftypes.NewValue(tftypes.String, nil), `{"type":"string"}`, false},
		{"default avro, invalid json", tftypes.NewValue(tftypes.String, nil), `not json`, true},
		{"json, invalid", tftypes.NewValue(tftypes.String, "JSON"), `not json`, true},
		{"protobuf, not json", tftypes.NewValue(tftypes.String, "PROTOBUF"), fixtureProtobuf1, false},
		{"unknown type skipped", tftypes.NewValue(tftypes.String, tftypes.UnknownValue), `not json`, false},
	}
	r := &schemaResource{}
	schemaResp := &fwresource.SchemaResponse{}
	r.Schema(context.Background(), fwresource.SchemaRequest{}, schemaResp)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := map[string]tftypes.Value{}
			for name, a := range schemaResp.Schema.Attributes {
				raw[name] = tftypes.NewValue(a.GetType().TerraformType(context.Background()), nil)
			}
			for name, b := range schemaResp.Schema.Blocks {
				raw[name] = tftypes.NewValue(b.Type().TerraformType(context.Background()), nil)
			}
			raw["subject"] = tftypes.NewValue(tftypes.String, "s")
			raw["schema"] = tftypes.NewValue(tftypes.String, tc.schema)
			raw["schema_type"] = tc.schemaType
			config := tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(context.Background()), raw)}
			resp := &fwresource.ValidateConfigResponse{}
			r.ValidateConfig(context.Background(), fwresource.ValidateConfigRequest{Config: config}, resp)
			if got := resp.Diagnostics.HasError(); got != tc.wantErr {
				t.Errorf("error = %v, want %v: %v", got, tc.wantErr, resp.Diagnostics)
			}
		})
	}
}

func TestProviderValidateConfigAuth(t *testing.T) {
	p := New("test")()
	schemaResp := &provider.SchemaResponse{}
	p.Schema(context.Background(), provider.SchemaRequest{}, schemaResp)

	cases := []struct {
		name    string
		set     map[string]string
		wantErr bool
	}{
		{"none", nil, false},
		{"basic", map[string]string{"username": "u", "password": "p"}, false},
		{"bearer", map[string]string{"bearer_token": "t"}, false},
		{"username only with bearer (accepted today)", map[string]string{"username": "u", "bearer_token": "t"}, false},
		{"basic and bearer", map[string]string{"username": "u", "password": "p", "bearer_token": "t"}, true},
		{"bearer and oauth2", map[string]string{"bearer_token": "t", "oauth2_token_url": "x", "oauth2_client_id": "i", "oauth2_client_secret": "s"}, true},
		{"partial oauth2 with bearer (accepted today)", map[string]string{"bearer_token": "t", "oauth2_token_url": "x"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := map[string]tftypes.Value{}
			for name, a := range schemaResp.Schema.Attributes {
				raw[name] = tftypes.NewValue(a.GetType().TerraformType(context.Background()), nil)
			}
			for k, v := range tc.set {
				raw[k] = tftypes.NewValue(tftypes.String, v)
			}
			config := tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(context.Background()), raw)}
			resp := &provider.ValidateConfigResponse{}
			p.(provider.ProviderWithValidateConfig).ValidateConfig(context.Background(), provider.ValidateConfigRequest{Config: config}, resp)
			if got := resp.Diagnostics.HasError(); got != tc.wantErr {
				t.Errorf("error = %v, want %v: %v", got, tc.wantErr, resp.Diagnostics)
			}
		})
	}
}
