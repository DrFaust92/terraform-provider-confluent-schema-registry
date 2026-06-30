package schemaregistry

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/riferrei/srclient"
)

var (
	_ datasource.DataSource              = &schemaDataSource{}
	_ datasource.DataSourceWithConfigure = &schemaDataSource{}
)

func newSchemaDataSource() datasource.DataSource {
	return &schemaDataSource{}
}

type schemaDataSource struct {
	client *srclient.SchemaRegistryClient
}

type schemaDataSourceModel struct {
	ID            types.String           `tfsdk:"id"`
	Subject       types.String           `tfsdk:"subject"`
	Version       types.Int64            `tfsdk:"version"`
	SchemaID      types.Int64            `tfsdk:"schema_id"`
	Schema        types.String           `tfsdk:"schema"`
	Compatibility types.String           `tfsdk:"compatibility"`
	References    []schemaReferenceModel `tfsdk:"references"`
}

func (d *schemaDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schema"
}

func (d *schemaDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a schema and its metadata from the Schema Registry by subject (optionally at a specific version).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The subject of the schema.",
				Computed:            true,
			},
			"subject": schema.StringAttribute{
				MarkdownDescription: "The subject related to the schema.",
				Required:            true,
			},
			"version": schema.Int64Attribute{
				MarkdownDescription: "The version of the schema. If omitted, the latest version is returned.",
				Optional:            true,
				Computed:            true,
			},
			"schema_id": schema.Int64Attribute{
				MarkdownDescription: "The schema ID.",
				Computed:            true,
			},
			"schema": schema.StringAttribute{
				MarkdownDescription: "The schema string.",
				Computed:            true,
			},
			"compatibility": schema.StringAttribute{
				MarkdownDescription: "The compatibility level of the subject.",
				Optional:            true,
			},
			"references": schema.ListNestedAttribute{
				MarkdownDescription: "The referenced schema names list.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "The referenced schema name.",
							Computed:            true,
						},
						"subject": schema.StringAttribute{
							MarkdownDescription: "The subject related to the schema.",
							Computed:            true,
						},
						"version": schema.Int64Attribute{
							MarkdownDescription: "The version of the schema.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *schemaDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*srclient.SchemaRegistryClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *srclient.SchemaRegistryClient, got: %T.", req.ProviderData))
		return
	}
	d.client = client
}

func (d *schemaDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config schemaDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	subject := config.Subject.ValueString()

	var sch *srclient.Schema
	var err error
	if config.Version.ValueInt64() > 0 {
		sch, err = d.client.GetSchemaByVersion(subject, int(config.Version.ValueInt64()))
	} else {
		sch, err = d.client.GetLatestSchema(subject)
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to read schema", err.Error())
		return
	}

	config.ID = types.StringValue(subject)
	config.Schema = types.StringValue(sch.Schema())
	config.SchemaID = types.Int64Value(int64(sch.ID()))
	config.Version = types.Int64Value(int64(sch.Version()))
	config.References = flattenReferences(sch.References())
	// compatibility is a passthrough of the configured value.

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
