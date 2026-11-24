package schemaregistry

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/riferrei/srclient"
)

var (
	_ list.ListResource              = &schemaListResource{}
	_ list.ListResourceWithConfigure = &schemaListResource{}
)

func newSchemaListResource() list.ListResource {
	return &schemaListResource{}
}

// schemaListResource lists schemaregistry_schema resources, one per subject.
type schemaListResource struct {
	client *srclient.SchemaRegistryClient
}

type schemaListConfigModel struct {
	Prefix types.String `tfsdk:"prefix"`
}

func (r *schemaListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	// Must match the managed resource type name being listed.
	resp.TypeName = req.ProviderTypeName + "_schema"
}

func (r *schemaListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists the subjects registered in the Schema Registry, one `schemaregistry_schema` per subject. Use with `terraform query` (Terraform 1.14 and later).",
		Attributes: map[string]listschema.Attribute{
			"prefix": listschema.StringAttribute{
				MarkdownDescription: "Only list subjects whose name starts with this prefix.",
				Optional:            true,
			},
		},
	}
}

func (r *schemaListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *schemaListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var config schemaListConfigModel
	if diags := req.Config.Get(ctx, &config); diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	if r.client == nil {
		result := req.NewListResult(ctx)
		result.Diagnostics.AddError("Client Error", "Provider client is not configured")
		stream.Results = slices.Values([]list.ListResult{result})
		return
	}

	subjects, err := r.client.GetSubjects()
	if err != nil {
		result := req.NewListResult(ctx)
		result.Diagnostics.AddError("Failed to list subjects", errorDetail(err))
		stream.Results = slices.Values([]list.ListResult{result})
		return
	}

	subjects = filterSubjects(subjects, config.Prefix.ValueString())
	reader := &schemaResource{client: r.client}

	results := make([]list.ListResult, 0, len(subjects))
	for _, subject := range subjects {
		result := req.NewListResult(ctx)
		result.DisplayName = subject
		result.Diagnostics.Append(result.Identity.Set(ctx, schemaResourceIdentityModel{Subject: types.StringValue(subject)})...)

		if req.IncludeResource {
			model := schemaResourceModel{ID: types.StringValue(subject)}
			if found := reader.readInto(&model, &result.Diagnostics); !found && !result.Diagnostics.HasError() {
				// Deleted between listing and reading; skip it.
				continue
			}
			result.Diagnostics.Append(result.Resource.Set(ctx, model)...)
		}

		results = append(results, result)
	}

	stream.Results = slices.Values(results)
}

// filterSubjects returns the subjects starting with prefix, sorted for a
// stable result order.
func filterSubjects(subjects []string, prefix string) []string {
	out := slices.DeleteFunc(slices.Clone(subjects), func(s string) bool { return !strings.HasPrefix(s, prefix) })
	slices.Sort(out)
	return out
}
