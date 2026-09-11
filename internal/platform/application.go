package platform

// ApplicationErrorCode is the stable classification for inspecting a
// user-selected application bundle. Callers branch on Code, never NativeCode.
type ApplicationErrorCode string

const (
	ApplicationInvalidArgument ApplicationErrorCode = "invalid_argument"
	ApplicationABIMismatch     ApplicationErrorCode = "abi_mismatch"
	ApplicationUnsupported     ApplicationErrorCode = "unsupported"
	ApplicationNotApplication  ApplicationErrorCode = "not_application"
	ApplicationNative          ApplicationErrorCode = "native"
)

func (c ApplicationErrorCode) Valid() bool {
	switch c {
	case ApplicationInvalidArgument,
		ApplicationABIMismatch,
		ApplicationUnsupported,
		ApplicationNotApplication,
		ApplicationNative:
		return true
	default:
		return false
	}
}

// ApplicationError describes a failed application-bundle inspection.
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
