package schemaregistry

import (
	"context"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/customdiff"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/structure"
	"github.com/riferrei/srclient"
)

func resourceSchema() *schema.Resource {
	return &schema.Resource{
		CreateContext: schemaCreate,
		UpdateContext: schemaUpdate,
		ReadContext:   schemaRead,
		DeleteContext: schemaDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		CustomizeDiff: customdiff.ComputedIf("version", func(ctx context.Context, d *schema.ResourceDiff, meta interface{}) bool {
			oldState, newState := d.GetChange("schema")
			newJSON, _ := structure.NormalizeJsonString(newState)
			oldJSON, _ := structure.NormalizeJsonString(oldState)
			schemaHasChange := newJSON != oldJSON

			// explicitly set a version change on schema change and make dependencies aware of a
			// version changed at `plan` time (computed field)
			return schemaHasChange || d.HasChange("version")
		}),
		Schema: map[string]*schema.Schema{
			"subject": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The subject related to the schema",
				ForceNew:    true,
			},
			"schema": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The schema string",
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					newJSON, _ := structure.NormalizeJsonString(new)
					oldJSON, _ := structure.NormalizeJsonString(old)
					return newJSON == oldJSON
				},
			},
			"schema_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The ID of the schema",
			},
			"version": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The schema version",
			},
			"compatibility": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The compatibility level of the subject",
			},
			"reference": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "The referenced schema list",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "The referenced schema name",
						},
						"subject": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "The referenced schema subject",
						},
						"version": {
							Type:        schema.TypeInt,
							Required:    true,
							Description: "The referenced schema version",
						},
					},
				},
			},
		},
	}
}

func schemaCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	subject := d.Get("subject").(string)
	schemaString := d.Get("schema").(string)
	references := ToRegistryReferences(d.Get("reference").([]interface{}))

	client := meta.(*srclient.SchemaRegistryClient)

	// CreateSchema's response only carries the schema ID, not the version, so the
	// computed attributes are populated by reading the subject back below.
	if _, err := client.CreateSchema(subject, schemaString, srclient.Avro, references...); err != nil {
		return diag.FromErr(err)
	}

	// Set compatibility level if user provided one
	if compatibility, ok := d.GetOk("compatibility"); ok {
		if _, err := client.ChangeSubjectCompatibilityLevel(subject, srclient.CompatibilityLevel(compatibility.(string))); err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(formatSchemaVersionID(subject))

	return schemaRead(ctx, d, meta)
}

func schemaUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	subject := d.Get("subject").(string)
	schemaString := d.Get("schema").(string)
	references := ToRegistryReferences(d.Get("reference").([]interface{}))

	client := meta.(*srclient.SchemaRegistryClient)

	if _, err := client.CreateSchema(subject, schemaString, srclient.Avro, references...); err != nil {
		// 42201 is schema incompatible error code from Confluent Schema Registry
		// https://docs.confluent.io/platform/current/schema-registry/develop/api.html#post--subjects-(string-%20subject)-versions
		if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "42201") {
			return diag.Errorf(`invalid "schema": incompatible`)
		}
		return diag.FromErr(err)
	}

	// Update compatibility level if it was changed
	if d.HasChange("compatibility") {
		if compatibility, ok := d.GetOk("compatibility"); ok {
			if _, err := client.ChangeSubjectCompatibilityLevel(subject, srclient.CompatibilityLevel(compatibility.(string))); err != nil {
				return diag.FromErr(err)
			}
		}
	}

	// The computed attributes (version in particular) are populated by reading
	// the subject back: CreateSchema's response does not carry the version.
	return schemaRead(ctx, d, meta)
}

func schemaRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(*srclient.SchemaRegistryClient)
	subject := extractSchemaVersionID(d.Id())

	schema, err := client.GetLatestSchema(subject)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			log.Printf("[WARN] Schema (%s) not found, removing from state", d.Id())
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	compatibility, err := client.GetCompatibilityLevel(subject, true)
	if err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("schema", schema.Schema()); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("schema_id", schema.ID()); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("subject", subject); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("version", schema.Version()); err != nil {
		return diag.FromErr(err)
	}

	if err = d.Set("reference", FromRegistryReferences(schema.References())); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("compatibility", compatibility); err != nil {
		return diag.FromErr(err)
	}

	return diags
}

func schemaDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(*srclient.SchemaRegistryClient)
	subject := extractSchemaVersionID(d.Id())

	// since 0.7.4 we need to first soft delete the schema and then hard delete it
	err := client.DeleteSubject(subject, false)
	if err != nil {
		return diag.FromErr(err)
	}
	err = client.DeleteSubject(subject, true)
	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

func FromRegistryReferences(references []srclient.Reference) []interface{} {
	if len(references) == 0 {
		return make([]interface{}, 0)
	}

	refs := make([]interface{}, 0, len(references))
	for _, reference := range references {
		refs = append(refs, map[string]interface{}{
			"name":    reference.Name,
			"subject": reference.Subject,
			"version": reference.Version,
		})
	}

	return refs
}

func ToRegistryReferences(references []interface{}) []srclient.Reference {

	if len(references) == 0 {
		return make([]srclient.Reference, 0)
	}

	refs := make([]srclient.Reference, 0, len(references))
	for _, reference := range references {
		r := reference.(map[string]interface{})

		refs = append(refs, srclient.Reference{
			Name:    r["name"].(string),
			Subject: r["subject"].(string),
			Version: r["version"].(int),
		})
	}

	return refs
}
