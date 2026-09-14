//go:build darwin && cgo

package darwin

/*
#cgo CFLAGS: -I${SRCDIR}/../../../native/include
#include <stdlib.h>
#include "daygo_application.h"
*/
import "C"

import (
	"context"
	"errors"
	"unsafe"

	"github.com/Jwz-git/Daygo/internal/platform"
)

const (
	applicationIdentifierBufferSize = 4096
	applicationNameBufferSize       = 4096
	applicationIconBufferSize       = 256 * 1024
)

func inspectApplication(ctx context.Context, path string) (platform.ApplicationIdentity, error) {
	pathData := C.CBytes([]byte(path))
	if pathData == nil {
		return platform.ApplicationIdentity{}, &platform.ApplicationError{Code: platform.ApplicationNative}
	}
	defer C.free(pathData)
	view := C.dg_application_string_view_v1{
		data: (*C.uint8_t)(pathData),
		len:  C.uint64_t(len(path)),
	}
	return callApplication(ctx, func(major C.uint32_t, info *C.dg_application_info_v2, nativeError *C.dg_application_error_v1) C.int32_t {
		return C.dg_application_inspect(major, view, info, nativeError)
	})
}

func lookupApplication(ctx context.Context, identifier string) (platform.ApplicationIdentity, error) {
	identity, err := lookupApplicationViaABI(ctx, identifier)
	if err == nil {
		return identity, nil
	}

	// LaunchServices registration and a bundle's own Info.plist can disagree
	// on identifier casing (com.apple.news vs com.apple.NEWS): the by-
	// identifier ABI fails with not_found for apps the filesystem walk clearly
	// found. Fall back to locating the bundle on disk and inspecting by path,
	// which resolves the icon the same way every other app gets one.
	var appErr *platform.ApplicationError
	if errors.As(err, &appErr) && appErr.Code == platform.ApplicationNotFound {
		if path, pathErr := findBundlePathByIdentifier(ctx, identifier); pathErr == nil && path != "" {
			return inspectApplication(ctx, path)
		}
	}
	return identity, err
}

// lookupApplicationViaABI resolves the identifier through LaunchServices.
func lookupApplicationViaABI(ctx context.Context, identifier string) (platform.ApplicationIdentity, error) {
	identifierData := C.CBytes([]byte(identifier))
	if identifierData == nil {
		return platform.ApplicationIdentity{}, &platform.ApplicationError{Code: platform.ApplicationNative}
	}
	defer C.free(identifierData)
	view := C.dg_application_string_view_v1{
		data: (*C.uint8_t)(identifierData),
		len:  C.uint64_t(len(identifier)),
	}
	return callApplication(ctx, func(major C.uint32_t, info *C.dg_application_info_v2, nativeError *C.dg_application_error_v1) C.int32_t {
		return C.dg_application_lookup(major, view, info, nativeError)
	})
}

// callApplication owns the caller-side buffers of one application ABI call and
// maps the native result onto the platform port. The buffers are freed before
// return; nothing native survives the call.
func callApplication(
	ctx context.Context,
	call func(C.uint32_t, *C.dg_application_info_v2, *C.dg_application_error_v1) C.int32_t,
) (platform.ApplicationIdentity, error) {
	var libraryMajor, libraryMinor C.uint32_t
	C.dg_application_abi_version(&libraryMajor, &libraryMinor)
	if uint32(libraryMajor) != uint32(C.DG_APPLICATION_ABI_MAJOR) {
		return platform.ApplicationIdentity{}, &platform.ApplicationError{Code: platform.ApplicationABIMismatch}
	}

	identifierData := C.malloc(applicationIdentifierBufferSize)
	nameData := C.malloc(applicationNameBufferSize)
	iconData := C.malloc(applicationIconBufferSize)
	if identifierData == nil || nameData == nil || iconData == nil {
		C.free(identifierData)
		C.free(nameData)
		C.free(iconData)
		return platform.ApplicationIdentity{}, &platform.ApplicationError{Code: platform.ApplicationNative}
	}
	defer C.free(identifierData)
	defer C.free(nameData)
	defer C.free(iconData)

	info := C.dg_application_info_v2{
		struct_size: C.uint32_t(C.sizeof_dg_application_info_v2),
		identifier: C.dg_application_buffer_v1{
			data:     (*C.uint8_t)(identifierData),
			capacity: applicationIdentifierBufferSize,
		},
		name: C.dg_application_buffer_v1{
			data:     (*C.uint8_t)(nameData),
			capacity: applicationNameBufferSize,
		},
		icon_png: C.dg_application_buffer_v1{
			data:     (*C.uint8_t)(iconData),
			capacity: applicationIconBufferSize,
		},
	}
	nativeError := C.dg_application_error_v1{struct_size: C.uint32_t(C.sizeof_dg_application_error_v1)}
	status := call(C.uint32_t(C.DG_APPLICATION_ABI_MAJOR), &info, &nativeError)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return platform.ApplicationIdentity{}, ctxErr
	}
	if status != C.DG_APPLICATION_OK {
		return platform.ApplicationIdentity{}, mapApplicationError(status, nativeError)
	}
	if info.identifier.len == 0 || info.identifier.len > applicationIdentifierBufferSize ||
		info.name.len == 0 || info.name.len > applicationNameBufferSize ||
		info.icon_png.len > applicationIconBufferSize {
		return platform.ApplicationIdentity{}, &platform.ApplicationError{Code: platform.ApplicationNative}
	}

	identity := platform.ApplicationIdentity{
		ID:   C.GoStringN((*C.char)(unsafe.Pointer(identifierData)), C.int(info.identifier.len)),
		Name: C.GoStringN((*C.char)(unsafe.Pointer(nameData)), C.int(info.name.len)),
	}
	if info.icon_png.len > 0 {
		identity.IconPNG = C.GoBytes(unsafe.Pointer(iconData), C.int(info.icon_png.len))
	}
	return identity, nil
}

func mapApplicationError(status C.int32_t, nativeError C.dg_application_error_v1) error {
	code := platform.ApplicationNative
	switch status {
	case C.DG_APPLICATION_E_INVALID_ARGUMENT:
		code = platform.ApplicationInvalidArgument
	case C.DG_APPLICATION_E_ABI_MISMATCH:
		code = platform.ApplicationABIMismatch
	case C.DG_APPLICATION_E_UNSUPPORTED:
		code = platform.ApplicationUnsupported
	case C.DG_APPLICATION_E_NOT_APPLICATION:
		code = platform.ApplicationNotApplication
	case C.DG_APPLICATION_E_NOT_FOUND:
		code = platform.ApplicationNotFound
	}
	return &platform.ApplicationError{Code: code, NativeCode: int64(nativeError.native_code)}
}
