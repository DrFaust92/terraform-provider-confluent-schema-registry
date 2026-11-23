package main

import (
	"context"
	"flag"
	"log"

	"terraform-provider-schema-registry/schemaregistry"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

//go:generate terraform fmt -recursive ./examples/
//go:generate go tool tfplugindocs generate --provider-name schemaregistry

// version is set at build/release time via ldflags.
var version = "dev"

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), schemaregistry.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/drfaust92/confluent-schema-registry",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
