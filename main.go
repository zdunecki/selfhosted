package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/zdunecki/selfhosted/cmd"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
