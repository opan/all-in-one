package main

import (
	"fmt"
	"os"

	listingCmd "github.com/all-in-one/cmd/listing"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "all-in-one",
	Short: "🚀 All-in-One Service Platform",
	Long: `🚀 All-in-One Service Platform
=============================

A comprehensive service platform that provides multiple microservices
in a single application for development and deployment convenience.`,
}

var listingCommand = &cobra.Command{
	Use:   "listing",
	Short: "Start the listing service",
	Long:  "🏷️  Launch the listing service to manage and serve listing data",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🏷️  Launching Listing Service...")
		listingCmd.Run()
	},
}

func main() {
	// Setup commands
	rootCmd.AddCommand(listingCommand)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
