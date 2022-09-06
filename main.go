package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/vercel/terraform-provider-preset/preset"
)

func main() {
	err := providerserver.Serve(context.Background(), preset.New, providerserver.ServeOpts{
		Address: "registry.terraform.io/vercel/preset",
	})

	if err != nil {
		log.Fatalf("unable to serve provider: %s", err)
	}
}
