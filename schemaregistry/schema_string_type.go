package schemaregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// schemaStringType is the type of the `schema` attribute. It is stored as a
// plain string (identical state to the previous jsontypes.Normalized type).
// Semantic equality normalizes JSON (AVRO and JSON schemas) exactly like
// jsontypes.Normalized and compares non-JSON (PROTOBUF) schemas ignoring
// whitespace differences.
type schemaStringType struct {
	basetypes.StringType
}

var _ basetypes.StringTypable = schemaStringType{}

func (t schemaStringType) String() string { return "schemaStringType" }

func (t schemaStringType) Equal(o attr.Type) bool {
	other, ok := o.(schemaStringType)
	if !ok {
		return false
	}
	return t.StringType.Equal(other.StringType)
}

func (t schemaStringType) ValueType(_ context.Context) attr.Value { return schemaString{} }

func (t schemaStringType) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return schemaString{StringValue: in}, nil
}

func (t schemaStringType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type of %T", attrValue)
	}
	return schemaString{StringValue: stringValue}, nil
}

// schemaString is the value of the `schema` attribute.
type schemaString struct {
	basetypes.StringValue
}

var _ basetypes.StringValuableWithSemanticEquals = schemaString{}

func newSchemaString(s string) schemaString {
	return schemaString{StringValue: basetypes.NewStringValue(s)}
}

func (v schemaString) Type(_ context.Context) attr.Type { return schemaStringType{} }

func (v schemaString) Equal(o attr.Value) bool {
	other, ok := o.(schemaString)
	if !ok {
		return false
	}
	return v.StringValue.Equal(other.StringValue)
}

func (v schemaString) StringSemanticEquals(_ context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	newValue, ok := newValuable.(schemaString)
	if !ok {
		diags.AddError("Semantic Equality Check Error", fmt.Sprintf("Expected value type %T, got %T. Please report this to the provider developers.", v, newValuable))
		return false, diags
	}
	return schemaStringsEqual(v.ValueString(), newValue.ValueString()), diags
}

// schemaStringsEqual reports whether two schema strings are equivalent: as
// normalized JSON when both parse as JSON, otherwise ignoring whitespace.
func schemaStringsEqual(a, b string) bool {
	na, errA := normalizeJSONString(a)
	nb, errB := normalizeJSONString(b)
	if errA == nil && errB == nil {
		return na == nb
	}
	return strings.Join(strings.Fields(a), " ") == strings.Join(strings.Fields(b), " ")
}

// normalizeJSONString mirrors jsontypes.Normalized's normalization so AVRO and
// JSON schemas compare exactly as before.
func normalizeJSONString(s string) (string, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var temp any
	if err := dec.Decode(&temp); err != nil {
		return "", err
	}
	b, err := json.Marshal(&temp)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
