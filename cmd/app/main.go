package main

import (
	"flag"
	"os"

	"github.com/x0k/medods-authentication-service/internal/app"
)

func main() {
	var config_path string
	flag.StringVar(&config_path, "config", os.Getenv("CONFIG_PATH"), "Config path")
	flag.Parse()
	if config_path == "" {
		config_path = ".env"
	}
	app.Run(config_path)
}
