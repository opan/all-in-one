package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	fmt.Println("🚀 All-in-One Service Platform")
	fmt.Println("=============================")

	// Check if a service is specified
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <service>")
		fmt.Println("Available services:")
		fmt.Println("  listing  - Start the listing service")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  go run main.go listing")
		fmt.Println()
		fmt.Println("Or run services directly:")
		fmt.Println("  go run cmd/listing/main.go")
		os.Exit(1)
	}

	service := os.Args[1]

	switch service {
	case "listing":
		fmt.Println("🏷️  Launching Listing Service...")
		cmd := exec.Command("go", "run", "cmd/listing/main.go")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			fmt.Printf("Error running listing service: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown service: %s\n", service)
		fmt.Println("Available services: listing")
		os.Exit(1)
	}
}
