package platform

// ApplicationErrorCode is the stable classification for resolving an
// application bundle. Callers branch on Code, never NativeCode.
type ApplicationErrorCode string

const (
	ApplicationInvalidArgument ApplicationErrorCode = "invalid_argument"
	ApplicationABIMismatch     ApplicationErrorCode = "abi_mismatch"
	ApplicationUnsupported     ApplicationErrorCode = "unsupported"
	ApplicationNotApplication  ApplicationErrorCode = "not_application"
	ApplicationNotFound        ApplicationErrorCode = "not_found"
	ApplicationNative          ApplicationErrorCode = "native"
)

func (c ApplicationErrorCode) Valid() bool {
	switch c {
	case ApplicationInvalidArgument,
		ApplicationABIMismatch,
		ApplicationUnsupported,
		ApplicationNotApplication,
		ApplicationNotFound,
		ApplicationNative:
		return true
	default:
		return false
	}
}

// ApplicationError describes a failed application-bundle resolution.
// NativeCode is numeric local diagnostics only and must not cross the UI API.
type ApplicationError struct {
	Code       ApplicationErrorCode
	NativeCode int64
}

func (e *ApplicationError) Error() string {
	if e == nil {
		return "application inspection failed"
	}
	return "application inspection failed: " + string(e.Code)
}
