package mcp

import (
	"context"
	"os"

	"github.com/Jwz-git/Daygo/internal/agentbridge"
	"github.com/Jwz-git/Daygo/internal/agentread"
)

// ServeStdio is the `daygo mcp` entrypoint. It opens the read-only database and
// the agent.sock write client, then serves MCP over stdin/stdout until EOF.
// Reads use the same read-only path as the CLI; writes cross the socket to the
// resident writer — this process opens no write path to the database
// (docs/decisions/agent-mcp-transport.md).
func ServeStdio(ctx context.Context) error {
	reader, err := agentread.Open(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	sock, err := agentread.SocketPath()
	if err != nil {
		return err
	}
	srv := NewServer(reader, agentbridge.NewClient(sock), os.Stdin, os.Stdout)
	return srv.Run(ctx)
}
