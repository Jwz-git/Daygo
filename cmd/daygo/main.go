package main

import (
	"context"
	"log"
	"os"

	"github.com/Jwz-git/Daygo/internal/agentcli"
	"github.com/Jwz-git/Daygo/internal/app"
	"github.com/Jwz-git/Daygo/internal/mcp"
)

func main() {
	if len(os.Args) > 1 {
		switch {
		case os.Args[1] == "mcp":
			if err := mcp.ServeStdio(context.Background()); err != nil {
				log.Fatal(err)
			}
			return
		case agentcli.Handles(os.Args[1]):
			os.Exit(agentcli.Main(os.Args[1:]))
		}
	}
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
