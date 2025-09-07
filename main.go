package main

import (
	"fmt"
	"os"

	listingCmd "github.com/all-in-one/cmd/listing"
)

func main() {
	fmt.Println("🚀 All-in-One Service Platform")
	fmt.Println("=============================")

	// Check if a service is specified
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./all-in-one <service>")
		fmt.Println("Available services:")
		fmt.Println("  listing  - Start the listing service")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  ./all-in-one listing")
		fmt.Println()
		fmt.Println("Development:")
		fmt.Println("  go run main.go listing")
		os.Exit(1)
	}

	service := os.Args[1]

	switch service {
	case "listing":
		fmt.Println("🏷️  Launching Listing Service...")
		listingCmd.Run()
	default:
		fmt.Printf("Unknown service: %s\n", service)
		fmt.Println("Available services: listing")
		os.Exit(1)
	}
}
