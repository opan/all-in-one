package main

import (
	"github.com/all-in-one/cmd/all-in-one/command"

	"github.com/rs/zerolog/log"
)

// @title           All-in-one API
// @version         1.0
// @description     All-in-one is a multi-functional apps with the main goal for learning
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1
// @schemes   http

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// @securityDefinitions.apikey DirectAuth
// @in header
// @name x-direct-auth-username
// @description Direct authentication bypass for testing (only works when DirectAuthEnabled is true in config). Provide username directly.

func main() {
	cmd := command.New()

	if err := cmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Command execution failed")
	}
}
