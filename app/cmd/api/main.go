package main

import (
	"github.com/uniscoot/scooter-renting-backend/app/internal/app"
)

var Version = "dev"

func main() {
	app.RunApi(Version)
}
