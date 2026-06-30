package schemaregistry

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/riferrei/srclient"
)

var (
	_ resource.Resource                = &schemaResource{}
	_ resource.ResourceWithConfigure   = &schemaResource{}
	_ resource.ResourceWithImportState = &schemaResource{}
)

func newSchemaResource() resource.Resource {
	return &schemaResource{}
}

type schemaResource struct {
	client *srclient.SchemaRegistryClient
}

type schemaReferenceModel struct {
	Name    types.String `tfsdk:"name"`
	Subject types.String `tfsdk:"subject"`
	Version types.Int64  `tfsdk:"version"`
}

type schemaResourceModel struct {
	ID            types.String           `tfsdk:"id"`
	Subject       types.String           `tfsdk:"subject"`
	Schema        jsontypes.Normalized   `tfsdk:"schema"`
	SchemaID      types.Int64            `tfsdk:"schema_id"`
	Version       types.Int64            `tfsdk:"version"`
	Compatibility types.String           `tfsdk:"compatibility"`
	Reference     []schemaReferenceModel `tfsdk:"reference"`
}

func (r *schemaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schema"
}

func (r *schemaResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a schema (and its versions) in the Schema Registry under a subject.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The subject of the schema.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"subject": schema.StringAttribute{
				MarkdownDescription: "The subject related to the schema.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"schema": schema.StringAttribute{
				MarkdownDescription: "The schema string.",
				Required:            true,
				CustomType:          jsontypes.NormalizedType{},
			},
			"schema_id": schema.Int64Attribute{
				MarkdownDescription: "The ID of the schema.",
				Computed:            true,
			},
			"version": schema.Int64Attribute{
				MarkdownDescription: "The schema version.",
				Computed:            true,
			},
			"compatibility": schema.StringAttribute{
				MarkdownDescription: "The compatibility level of the subject.",
				Optional:            true,
				Computed:            true,
			},
		},
		Blocks: map[string]schema.Block{
			"reference": schema.ListNestedBlock{
				MarkdownDescription: "The referenced schema list.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "The referenced schema name.",
							Required:            true,
						},
						"subject": schema.StringAttribute{
							MarkdownDescription: "The referenced schema subject.",
							Required:            true,
						},
						"version": schema.Int64Attribute{
							MarkdownDescription: "The referenced schema version.",
							Required:            true,
						},
					},
				},
			},
		},
	}
}

func (r *schemaResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*srclient.SchemaRegistryClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *srclient.SchemaRegistryClient, got: %T.", req.ProviderData))
		return
	}
	r.client = client
}

func (r *schemaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan schemaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	subject := plan.Subject.ValueString()
	if _, err := r.client.CreateSchema(subject, plan.Schema.ValueString(), srclient.Avro, expandReferences(plan.Reference)...); err != nil {
		resp.Diagnostics.AddError("Failed to create schema", err.Error())
		return
	}

	if !plan.Compatibility.IsNull() && !plan.Compatibility.IsUnknown() && plan.Compatibility.ValueString() != "" {
		if _, err := r.client.ChangeSubjectCompatibilityLevel(subject, srclient.CompatibilityLevel(plan.Compatibility.ValueString())); err != nil {
			resp.Diagnostics.AddError("Failed to set compatibility level", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(subject)
	if found := r.readInto(&plan, &resp.Diagnostics); resp.Diagnostics.HasError() {
		return
	} else if !found {
		resp.Diagnostics.AddError("Failed to read schema after create", fmt.Sprintf("subject %q not found immediately after creation", subject))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *schemaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state schemaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found := r.readInto(&state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *schemaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state schemaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	subject := plan.ID.ValueString()
	if _, err := r.client.CreateSchema(subject, plan.Schema.ValueString(), srclient.Avro, expandReferences(plan.Reference)...); err != nil {
		// 42201 / 409 is the incompatible-schema error from Confluent Schema Registry.
		if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "42201") {
			resp.Diagnostics.AddError("Invalid schema", `invalid "schema": incompatible`)
			return
		}
		resp.Diagnostics.AddError("Failed to update schema", err.Error())
		return
	}

	if !plan.Compatibility.Equal(state.Compatibility) && !plan.Compatibility.IsNull() && !plan.Compatibility.IsUnknown() && plan.Compatibility.ValueString() != "" {
		if _, err := r.client.ChangeSubjectCompatibilityLevel(subject, srclient.CompatibilityLevel(plan.Compatibility.ValueString())); err != nil {
			resp.Diagnostics.AddError("Failed to set compatibility level", err.Error())
			return
		}
	}

	if found := r.readInto(&plan, &resp.Diagnostics); resp.Diagnostics.HasError() {
		return
	} else if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *schemaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state schemaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	subject := state.ID.ValueString()
	// Since srclient 0.7.4 a subject must be soft-deleted then hard-deleted.
	if err := r.client.DeleteSubject(subject, false); err != nil {
		resp.Diagnostics.AddError("Failed to delete schema", err.Error())
		return
	}
	if err := r.client.DeleteSubject(subject, true); err != nil {
		resp.Diagnostics.AddError("Failed to delete schema", err.Error())
		return
	}
}

func (r *schemaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// readInto fetches the latest schema for m.ID's subject and populates m.
// Returns false (without diagnostics) when the subject no longer exists.
func (r *schemaResource) readInto(m *schemaResourceModel, diags *diag.Diagnostics) (found bool) {
	subject := m.ID.ValueString()

	sch, err := r.client.GetLatestSchema(subject)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return false
		}
		diags.AddError("Failed to read schema", err.Error())
		return false
	}

	compatibility, err := r.client.GetCompatibilityLevel(subject, true)
	if err != nil {
		diags.AddError("Failed to read compatibility level", err.Error())
		return false
	}

	m.Subject = types.StringValue(subject)
	m.Schema = jsontypes.NewNormalizedValue(sch.Schema())
	m.SchemaID = types.Int64Value(int64(sch.ID()))
	m.Version = types.Int64Value(int64(sch.Version()))
	m.Reference = flattenReferences(sch.References())
	if compatibility != nil {
		m.Compatibility = types.StringValue(string(*compatibility))
	}
	return true
}

func expandReferences(references []schemaReferenceModel) []srclient.Reference {
	refs := make([]srclient.Reference, 0, len(references))
	for _, ref := range references {
		refs = append(refs, srclient.Reference{
			Name:    ref.Name.ValueString(),
			Subject: ref.Subject.ValueString(),
			Version: int(ref.Version.ValueInt64()),
		})
	}
	return refs
}

func flattenReferences(references []srclient.Reference) []schemaReferenceModel {
	if len(references) == 0 {
		return nil
	}
	refs := make([]schemaReferenceModel, 0, len(references))
	for _, ref := range references {
		refs = append(refs, schemaReferenceModel{
			Name:    types.StringValue(ref.Name),
			Subject: types.StringValue(ref.Subject),
			Version: types.Int64Value(int64(ref.Version)),
		})
	}
	return refs
}
