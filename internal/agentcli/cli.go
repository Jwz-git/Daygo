// Package agentcli is the daygo CLI's read command surface (docs/05 §5.9.1).
// It dispatches the read commands onto internal/agentread, printing the
// schema_version-enveloped JSON with --json and a concise human summary
// otherwise, and maps failures onto the fixed exit codes.
package agentcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/Jwz-git/Daygo/internal/agentbridge"
	"github.com/Jwz-git/Daygo/internal/agentread"
)

// Commands is the read command set this build serves (docs/05 §5.9.1). search
// is deferred with its semantics (09 §9.1) and is not offered here.
var Commands = []string{"status", "timeline", "card", "daily", "weekly", "categories", "write"}

// Handles reports whether cmd is a CLI read command. cmd/daygo uses it to route
// only known commands here, so a bare launch (and stray GUI launch flags) fall
// through to the resident app rather than being treated as CLI input.
func Handles(cmd string) bool {
	return slices.Contains(Commands, cmd)
}

// exit codes, docs/05 §5.9.1.
const (
	exitOK         = 0
	exitUnexpected = 1
	exitUsage      = 2
	exitNotFound   = 3
	exitNoData     = 5
)

type options struct{ json bool }

// Main runs one CLI invocation and returns the process exit code. args is
// os.Args[1:] (command plus its arguments).
func Main(args []string) int {
	return run(context.Background(), args, os.Stdout, os.Stderr)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitUsage
	}
	cmd := args[0]
	rest, opts := parseFlags(args[1:])

	switch cmd {
	case "write":
		if len(rest) != 2 || !slices.Contains(agentbridge.Operations, rest[0]) || !json.Valid([]byte(rest[1])) {
			return emitError(stderr, opts, faultUsage("usage: daygo write <operation> '<json arguments>'"))
		}
		var args json.RawMessage = []byte(rest[1])
		socket, err := agentread.SocketPath()
		if err != nil {
			return emitError(stderr, opts, err)
		}
		data, err := agentbridge.NewClient(socket).Do(ctx, rest[0], args, agentbridge.SourceCLI)
		if err != nil {
			var bridgeErr *agentbridge.Error
			if errors.As(err, &bridgeErr) {
				return emitError(stderr, opts, faultOfCode(bridgeErr.Code, bridgeErr.Message))
			}
			return emitError(stderr, opts, err)
		}
		if opts.json {
			return emitResult(stdout, opts, struct {
				SchemaVersion int             `json:"schema_version"`
				Data          json.RawMessage `json:"data"`
			}{SchemaVersion: agentread.SchemaVersion, Data: data})
		}
		fmt.Fprintln(stdout, "ok")
		return exitOK
	case "status":
		return withReader(ctx, opts, stdout, stderr, func(r *agentread.Reader) (any, error) {
			return r.Status(ctx)
		})
	case "timeline":
		day := optionalArg(rest)
		return withReader(ctx, opts, stdout, stderr, func(r *agentread.Reader) (any, error) {
			return r.Timeline(ctx, day)
		})
	case "card":
		id, err := requireIntArg(rest)
		if err != nil {
			return emitError(stderr, opts, err)
		}
		return withReader(ctx, opts, stdout, stderr, func(r *agentread.Reader) (any, error) {
			return r.Card(ctx, id)
		})
	case "daily":
		day := optionalArg(rest)
		return withReader(ctx, opts, stdout, stderr, func(r *agentread.Reader) (any, error) {
			return r.Daily(ctx, day)
		})
	case "weekly":
		weekStart := optionalArg(rest)
		return withReader(ctx, opts, stdout, stderr, func(r *agentread.Reader) (any, error) {
			return r.Weekly(ctx, weekStart)
		})
	case "categories":
		return withReader(ctx, opts, stdout, stderr, func(r *agentread.Reader) (any, error) {
			return r.Categories(ctx)
		})
	default:
		fmt.Fprintf(stderr, "daygo: unknown command %q\n", cmd)
		usage(stderr)
		return exitUsage
	}
}
