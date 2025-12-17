package schemaregistry

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/riferrei/srclient"

	uuid "github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourceSchema_basic(t *testing.T) {
	dataSourceName := "data.schemaregistry_schema.test"
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	subject := fmt.Sprintf("sub%s", u)

	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviders,
		PreCheck:          func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: fixtureDataSourceSchemaBuild(subject, fixtureAvro1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "id", subject),
					resource.TestCheckResourceAttr(dataSourceName, "subject", subject),
					resource.TestCheckResourceAttr(dataSourceName, "version", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "schema", strings.ReplaceAll(fixtureAvro1, "\\", "")),
					resource.TestCheckResourceAttrSet(dataSourceName, "schema_id"),
				),
			},
		},
	})
}

func TestAccDataSourceSchemaReferences_basic(t *testing.T) {
	// GIVEN
	dataSourceName := "data.schemaregistry_schema.schemaWithReference"
	url, found := os.LookupEnv("SCHEMA_REGISTRY_URL")
	if !found {
		t.Fatalf("SCHEMA_REGISTRY_URL must be set for acceptance tests")
	}
	hasBasicAuth := os.Getenv("SCHEMA_REGISTRY_USERNAME") != "" && os.Getenv("SCHEMA_REGISTRY_PASSWORD") != ""
	hasBearerToken := os.Getenv("SCHEMA_REGISTRY_BEARER_TOKEN") != ""
	hasOAuth2 := os.Getenv("SCHEMA_REGISTRY_OAUTH2_TOKEN_URL") != "" &&
		os.Getenv("SCHEMA_REGISTRY_OAUTH2_CLIENT_ID") != "" &&
		os.Getenv("SCHEMA_REGISTRY_OAUTH2_CLIENT_SECRET") != ""

	if !hasBasicAuth && !hasBearerToken && !hasOAuth2 {
		t.Fatal("Either SCHEMA_REGISTRY_USERNAME and SCHEMA_REGISTRY_PASSWORD, or SCHEMA_REGISTRY_BEARER_TOKEN, or SCHEMA_REGISTRY_OAUTH2_TOKEN_URL/CLIENT_ID/CLIENT_SECRET must be set for acceptance tests")
	}

	client := srclient.CreateSchemaRegistryClient(url)
	if hasBasicAuth {
		username := os.Getenv("SCHEMA_REGISTRY_USERNAME")
		password :=  os.Getenv("SCHEMA_REGISTRY_PASSWORD")
		client.SetCredentials(username, password)
	}
	if hasBearerToken {
		token := os.Getenv("SCHEMA_REGISTRY_BEARER_TOKEN")
		client.SetBearerToken(token)
	}
	if hasOAuth2 {
		tokenURL := os.Getenv("SCHEMA_REGISTRY_OAUTH2_TOKEN_URL")
		clientID := os.Getenv("SCHEMA_REGISTRY_OAUTH2_CLIENT_ID")
		clientSecret := os.Getenv("SCHEMA_REGISTRY_OAUTH2_CLIENT_SECRET")

		token, err := getToken(tokenURL, clientID, clientSecret, nil)
		if err != nil {
			t.Fatalf("Failed to get OAuth2 token: %s", err)
		}
		client.SetBearerToken(token)
	}

	// AND
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}

	referencedSchemaSubject := fmt.Sprintf("referencedSub-%s", u)
	referencedSchema := strings.ReplaceAll(fixtureAvro1, "\\", "")

	schemaWithReferenceSubject := fmt.Sprintf("sub-%s", u)
	schemaWithReference := `["akc.test.userAdded"]`

	references := []srclient.Reference{
		{
			Name:    "akc.test.userAdded",
			Subject: referencedSchemaSubject,
			Version: 1,
		},
	}

	// AND
	if _, err = client.CreateSchema(referencedSchemaSubject, referencedSchema, srclient.Avro); err != nil {
		t.Fatalf("could not create schema for subject: %s, err: %s", referencedSchema, err)
	}

	if _, err = client.CreateSchema(schemaWithReferenceSubject, schemaWithReference, srclient.Avro, references...); err != nil {
		t.Fatalf("could not create schema for subject: %s, err: %s", referencedSchemaSubject, err)
	}

	// WHEN / THEN
	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviders,
		PreCheck:          func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					data "schemaregistry_schema" "schemaWithReference" {
						subject = "%s"
					}
				`, schemaWithReferenceSubject),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "id", schemaWithReferenceSubject),
					resource.TestCheckResourceAttr(dataSourceName, "subject", schemaWithReferenceSubject),
					resource.TestCheckResourceAttrSet(dataSourceName, "schema_id"),
					resource.TestCheckResourceAttr(dataSourceName, "version", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "schema", schemaWithReference),

					resource.TestCheckResourceAttr(dataSourceName, "references.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "references.0.name", references[0].Name),
					resource.TestCheckResourceAttr(dataSourceName, "references.0.subject", references[0].Subject),
					resource.TestCheckResourceAttr(dataSourceName, "references.0.version", strconv.Itoa(references[0].Version)),
				),
			},
		},
	})
}

func TestAccDataSourceSchema_atVersion(t *testing.T) {
	// GIVEN
	dataSourceName := "data.schemaregistry_schema.schemaAtVersion"
	url, found := os.LookupEnv("SCHEMA_REGISTRY_URL")
	if !found {
		t.Fatalf("SCHEMA_REGISTRY_URL must be set for acceptance tests")
	}
	hasBasicAuth := os.Getenv("SCHEMA_REGISTRY_USERNAME") != "" && os.Getenv("SCHEMA_REGISTRY_PASSWORD") != ""
	hasBearerToken := os.Getenv("SCHEMA_REGISTRY_BEARER_TOKEN") != ""
	hasOAuth2 := os.Getenv("SCHEMA_REGISTRY_OAUTH2_TOKEN_URL") != "" &&
		os.Getenv("SCHEMA_REGISTRY_OAUTH2_CLIENT_ID") != "" &&
		os.Getenv("SCHEMA_REGISTRY_OAUTH2_CLIENT_SECRET") != ""

	if !hasBasicAuth && !hasBearerToken && !hasOAuth2 {
		t.Fatal("Either SCHEMA_REGISTRY_USERNAME and SCHEMA_REGISTRY_PASSWORD, or SCHEMA_REGISTRY_BEARER_TOKEN, or SCHEMA_REGISTRY_OAUTH2_TOKEN_URL/CLIENT_ID/CLIENT_SECRET must be set for acceptance tests")
	}

	client := srclient.CreateSchemaRegistryClient(url)
	if hasBasicAuth {
		username := os.Getenv("SCHEMA_REGISTRY_USERNAME")
		password :=  os.Getenv("SCHEMA_REGISTRY_PASSWORD")
		client.SetCredentials(username, password)
	}
	if hasBearerToken {
		token := os.Getenv("SCHEMA_REGISTRY_BEARER_TOKEN")
		client.SetBearerToken(token)
	}
	if hasOAuth2 {
		tokenURL := os.Getenv("SCHEMA_REGISTRY_OAUTH2_TOKEN_URL")
		clientID := os.Getenv("SCHEMA_REGISTRY_OAUTH2_CLIENT_ID")
		clientSecret := os.Getenv("SCHEMA_REGISTRY_OAUTH2_CLIENT_SECRET")

		token, err := getToken(tokenURL, clientID, clientSecret, nil)
		if err != nil {
			t.Fatalf("Failed to get OAuth2 token: %s", err)
		}
		client.SetBearerToken(token)
	}

	// AND
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}

	referencedSchemaSubject := fmt.Sprintf("referencedSub-%s", u)
	referencedSchema := strings.ReplaceAll(fixtureAvro1, "\\", "")
	referencedSchemaLatest := strings.ReplaceAll(fixtureAvro2, "\\", "")

	// AND
	if _, err = client.CreateSchema(referencedSchemaSubject, referencedSchema, srclient.Avro); err != nil {
		t.Fatalf("could not create schema for subject: %s, err: %s", referencedSchema, err)
	}

	if _, err = client.CreateSchema(referencedSchemaSubject, referencedSchemaLatest, srclient.Avro); err != nil {
		t.Fatalf("could not create schema for subject: %s, err: %s", referencedSchema, err)
	}

	// WHEN / THEN
	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviders,
		PreCheck:          func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					data "schemaregistry_schema" "schemaAtVersion" {
						subject = "%s"
						version = 1
					}
				`, referencedSchemaSubject),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "id", referencedSchemaSubject),
					resource.TestCheckResourceAttr(dataSourceName, "subject", referencedSchemaSubject),
					resource.TestCheckResourceAttrSet(dataSourceName, "schema_id"),
					resource.TestCheckResourceAttr(dataSourceName, "version", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "schema", referencedSchema),
				),
			},
		},
	})
}

func TestAccDataSourceSchema_withCompatibility(t *testing.T) {
	dataSourceName := "data.schemaregistry_schema.test"
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	subject := fmt.Sprintf("sub%s", u)

	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviders,
		PreCheck:          func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: fixtureDataSourceSchemaBuildWithCompatibility(subject, fixtureAvro1, "BACKWARD"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "id", subject),
					resource.TestCheckResourceAttr(dataSourceName, "subject", subject),
					resource.TestCheckResourceAttr(dataSourceName, "version", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "schema", strings.ReplaceAll(fixtureAvro1, "\\", "")),
					resource.TestCheckResourceAttrSet(dataSourceName, "schema_id"),
					resource.TestCheckResourceAttr(dataSourceName, "compatibility", "BACKWARD"),
				),
			},
		},
	})
}

