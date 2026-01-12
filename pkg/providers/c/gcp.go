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
	"time"

	"github.com/zdunecki/selfhosted/pkg/providers"
)

func main() {
	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	gcp := providers.NewGCP()
	// No config needed; provider will resolve auth from SA JSON (if provided), ADC, or gcloud token.

	// Resolve auth using the provider instance (internally: SA JSON -> ADC -> gcloud token).
	ts, method, err := gcp.ResolveAuth()
	if err != nil {
		panic(err)
	}

	_ = ts // ResolveAuth caches token source inside gcp; ListProjects will use it.

	projects, err := gcp.ListProjects()
	if err != nil {
		panic(err)
	}

	fmt.Printf("auth=%s projects=%d\n", method, len(projects))
	for _, p := range projects {
		fmt.Printf("%-28s  %-24s  %s\n", p.ProjectID, p.DisplayName, p.State)
	}
}
