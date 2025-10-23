package main

import (
	"github.com/all-in-one/cmd/listing/command"
	"github.com/sirupsen/logrus"
)

func main() {
	cmd := command.New()

	if err := cmd.Execute(); err != nil {
		logrus.Fatalf("Command execution failed: %v", err)
	}
}
