package main

import (
	"github.com/all-in-one/cmd/listing/command"
	"github.com/sirupsen/logrus"
)

// @title Listing API
// @version 1.0
// @description API for managing listing items
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1
// @schemes http

func main() {
	cmd := command.New()

	if err := cmd.Execute(); err != nil {
		logrus.Fatalf("Command execution failed: %v", err)
	}
}
