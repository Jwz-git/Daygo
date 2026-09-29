package platform

import "errors"

// ErrCapabilityUnavailable reports that a System capability is not implemented
// on the current platform or build. An adapter returns it — directly or wrapped
// with %w — instead of pretending success, so a caller can distinguish "this
// platform will never do this" from a transient failure worth retrying with
// errors.Is(err, platform.ErrCapabilityUnavailable).
var ErrCapabilityUnavailable = errors.New("platform capability unavailable")
