package main

import (
	"log"

	"github.com/cng1985/nwatch/internal/app"
)

func main() {
	application := app.New()
	application.Run()
	if err := application.Err(); err != nil {
		log.Fatal(err)
	}
}
