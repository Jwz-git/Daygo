package agentcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Jwz-git/Daygo/internal/agentread"
)

// parseFlags splits the post-command tokens into positional arguments and
// options. Only --json is recognized; other tokens are positional so a stray
// flag surfaces as an invalid argument rather than being silently dropped.
func parseFlags(args []string) ([]string, options) {
	var rest []string
	var opts options
	for _, a := range args {
		if a == "--json" {
			opts.json = true
			continue
		}
		rest = append(rest, a)
	}
	return rest, opts
}

func optionalArg(rest []string) string {
	if len(rest) == 0 {
		return ""
	}
	return rest[0]
}

func requireIntArg(rest []string) (int64, error) {
	if len(rest) == 0 {
		return 0, faultUsage("card id is required")
	}
	id, err := strconv.ParseInt(rest[0], 10, 64)
	if err != nil {
		return 0, faultUsage("card id must be an integer")
	}
	return id, nil
}

// withReader opens the business database read-only, runs fn, and prints the
// result or maps the failure onto an exit code. loc is the host zone (nil →
// time.Local); the database path honors the DAYGO_DB override.
func withReader(ctx context.Context, opts options, stdout, stderr io.Writer, fn func(*agentread.Reader) (any, error)) int {
	reader, err := agentread.Open(ctx, nil)
	if err != nil {
		return emitError(stderr, opts, err)
	}
	defer reader.Close()

	result, err := fn(reader)
	if err != nil {
		return emitError(stderr, opts, err)
	}
	return emitResult(stdout, opts, result)
}

type errorEnvelope struct {
	SchemaVersion int       `json:"schema_version"`
	Error         errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func faultUsage(message string) error {
	return faultOfCode(agentread.CodeInvalidArgument, message)
}

// faultOfCode builds an agentread.Fault so CLI-originated argument errors carry
// the same code/message shape as reader failures.
func faultOfCode(code, message string) error {
	return &agentread.Fault{Code: code, Message: message}
}

func emitResult(stdout io.Writer, opts options, result any) int {
	if opts.json {
		b, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return exitUnexpected
		}
		fmt.Fprintf(stdout, "%s\n", b)
		return exitOK
	}
	renderText(stdout, result)
	return exitOK
}

func emitError(stderr io.Writer, opts options, err error) int {
	code, message := faultOf(err)
	if opts.json {
		b, mErr := json.MarshalIndent(errorEnvelope{
			SchemaVersion: agentread.SchemaVersion,
			Error:         errorBody{Code: code, Message: message},
		}, "", "  ")
		if mErr != nil {
			return exitUnexpected
		}
		fmt.Fprintf(stderr, "%s\n", b)
	} else {
		fmt.Fprintf(stderr, "daygo: %s: %s\n", code, message)
	}
	return exitFor(code)
}

// faultOf extracts the contract code/message, defaulting anything that is not
// an agentread.Fault to internal (an unexpected failure the caller should not
// have to interpret).
func faultOf(err error) (string, string) {
	var f *agentread.Fault
	if errors.As(err, &f) {
		return f.Code, f.Message
	}
	return agentread.CodeInternal, "unexpected error"
}

func exitFor(code string) int {
	switch code {
	case agentread.CodeInvalidArgument:
		return exitUsage
	case agentread.CodeNotFound:
		return exitNotFound
	case agentread.CodeNoData:
		return exitNoData
	default:
		return exitUnexpected
	}
}

func usage(w io.Writer) {
	fmt.Fprintf(w, "usage: daygo <command> [args] [--json]\n")
	fmt.Fprintf(w, "commands: %s\n", strings.Join(Commands, ", "))
}
