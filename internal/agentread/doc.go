// Package agentread is the read side of Daygo's external programmatic
// interface (docs/05 §5.9.1, docs/modules/agent.md). It opens the business
// database read-only and assembles the same timeline / card / daily / weekly /
// categories views the bindings serve, marshaled as the snake_case,
// schema_version-enveloped JSON the daygo CLI and the daygo mcp subprocess
// share.
//
// It is a separate process from the daemon: reads go through
// storage.OpenReadOnly (SQLITE_OPEN_READONLY + PRAGMA query_only), never the
// app layer, so this package imports storage / insight / timeutil / domain and
// nothing from internal/app or Wails. Writes are not here — they travel the
// agent.sock channel (internal/agentbridge), never a second database
// connection (docs/05 §5.9.2).
package agentread

import (
	"time"
)

// SchemaVersion is the value every top-level JSON object carries
// (docs/05 §5.9.1 rule 1, §5.10.1). It is frozen after the first public
// release: only optional fields may be added afterward.
const SchemaVersion = 1

// Fault is a command failure with a closed-set code and a sanitized message.
// The CLI maps Code to an exit code and prints it as the error envelope on
// stderr (docs/05 §5.9.1 rule 5); the MCP tool face maps it into its result
// envelope. Messages never carry file paths, keys, or LLM payloads.
type Fault struct {
	Code    string
	Message string
}

func (f *Fault) Error() string { return f.Code + ": " + f.Message }

// The closed set of read-side fault codes.
const (
	CodeInvalidArgument = "invalid_argument"
	CodeNotFound        = "not_found"
	CodeNoData          = "no_data"
	CodeInternal        = "internal"
)

func faultf(code, message string) *Fault { return &Fault{Code: code, Message: message} }

// formatTime renders a Unix second as the fixed docs/05 §5.9.1 rule 3 format
// (yyyy-MM-dd'T'HH:mm:ssZZZZZ), which is exactly time.RFC3339 in the host zone.
func formatTime(unixSec int64, loc *time.Location) string {
	if loc == nil {
		loc = time.Local
	}
	return time.Unix(unixSec, 0).In(loc).Format(time.RFC3339)
}
