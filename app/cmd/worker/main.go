// Worker entrypoint: subscribes to RabbitMQ and sends notification emails.
package main

import (
	"github.com/uniscoot/scooter-renting-backend/app/internal/app"
)

var Version = "dev"

func main() {
	app.RunWorker(Version)
}
