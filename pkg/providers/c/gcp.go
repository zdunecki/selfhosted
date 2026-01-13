//go:build ignore

// This file is a local debugging utility and is excluded from normal builds.
//
// Usage:
//
//	go run /Users/zdunecki/Code/zdunecki/selfhosted/pkg/providers/c/gcp.go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	serviceusage "cloud.google.com/go/serviceusage/apiv1"
	"cloud.google.com/go/serviceusage/apiv1/serviceusagepb"
	"github.com/zdunecki/selfhosted/pkg/providers"
)

func main() {
	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	gcp := providers.NewGCP()
	// No config needed; provider will resolve auth from SA JSON (if provided), ADC, or gcloud token.

	// Resolve auth using the provider instance (internally: SA JSON -> ADC -> gcloud token).
	_, _, err := gcp.ResolveAuth()
	if err != nil {
		panic(err)
	}

	fmt.Println("auth resolved")

	abc()

	// _ = ts // ResolveAuth caches token source inside gcp; ListProjects will use it.

	// projects, err := gcp.ListProjects()
	// if err != nil {
	// 	panic(err)
	// }

	// fmt.Printf("auth=%s projects=%d\n", method, len(projects))
	// for _, p := range projects {
	// 	fmt.Printf("%-28s  %-24s  %s\n", p.ProjectID, p.DisplayName, p.State)
	// }
}

func abc() {
	ctx := context.Background()

	// Create the client
	client, err := serviceusage.NewClient(ctx)
	if err != nil {
		fmt.Printf("Failed to create client: %v", err)
	}
	defer client.Close()

	projectID := "selfhosted-94489820"
	serviceName := "compute.googleapis.com"

	// Enable the API
	req := &serviceusagepb.EnableServiceRequest{
		Name: fmt.Sprintf("projects/%s/services/%s", projectID, serviceName),
	}

	op, err := client.EnableService(ctx, req)
	if err != nil {
		log.Fatalf("Failed to enable service: %v", err)
	}

	// Wait for the operation to complete
	resp, err := op.Wait(ctx)
	if err != nil {
		log.Fatalf("Failed to wait for operation: %v", err)
	}

	fmt.Printf("Service enabled: %v\n", resp)
}
