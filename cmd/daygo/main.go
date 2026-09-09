package main

import (
	"log"

	"github.com/Jwz-git/Daygo/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
