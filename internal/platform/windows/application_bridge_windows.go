//go:build windows && cgo

package windows

/*
#cgo CFLAGS: -DDAYGO_APPLICATION_STATIC -I${SRCDIR}/../../../native/include
#cgo LDFLAGS: ${SRCDIR}/../../../build/native/windows/amd64/libdaygo_capture.a
#cgo LDFLAGS: -luser32 -Wl,-Bstatic -lstdc++ -lwinpthread -Wl,-Bdynamic -lgcc -lgcc_eh
#include <stdlib.h>
#include "daygo_application.h"
*/
import "C"

import (
	"context"
	"unsafe"

	"github.com/Jwz-git/Daygo/internal/platform"
)

const (
	applicationIdentifierBufferSize = 4096
	applicationNameBufferSize       = 4096
	applicationIconBufferSize       = 256 * 1024
)

func inspectApplication(ctx context.Context, path string) (platform.ApplicationIdentity, error) {
	data := C.CBytes([]byte(path))
	if data == nil {
		return platform.ApplicationIdentity{}, &platform.ApplicationError{Code: platform.ApplicationNative}
	}
	defer C.free(data)
	view := C.dg_application_string_view_v1{data: (*C.uint8_t)(data), len: C.uint64_t(len(path))}
	return callApplication(ctx, func(major C.uint32_t, info *C.dg_application_info_v2, nativeError *C.dg_application_error_v1) C.int32_t {
		return C.dg_application_inspect(major, view, info, nativeError)
	})
}

func lookupApplication(ctx context.Context, identifier string) (platform.ApplicationIdentity, error) {
	data := C.CBytes([]byte(identifier))
	if data == nil {
		return platform.ApplicationIdentity{}, &platform.ApplicationError{Code: platform.ApplicationNative}
	}
	defer C.free(data)
	view := C.dg_application_string_view_v1{data: (*C.uint8_t)(data), len: C.uint64_t(len(identifier))}
	return callApplication(ctx, func(major C.uint32_t, info *C.dg_application_info_v2, nativeError *C.dg_application_error_v1) C.int32_t {
		return C.dg_application_lookup(major, view, info, nativeError)
	})
}

func callApplication(ctx context.Context, call func(C.uint32_t, *C.dg_application_info_v2, *C.dg_application_error_v1) C.int32_t) (platform.ApplicationIdentity, error) {
	var major, minor C.uint32_t
	C.dg_application_abi_version(&major, &minor)
	if uint32(major) != uint32(C.DG_APPLICATION_ABI_MAJOR) {
		return platform.ApplicationIdentity{}, &platform.ApplicationError{Code: platform.ApplicationABIMismatch}
	}
	identifier := C.malloc(applicationIdentifierBufferSize)
	name := C.malloc(applicationNameBufferSize)
	icon := C.malloc(applicationIconBufferSize)
	if identifier == nil || name == nil || icon == nil {
		C.free(identifier)
		C.free(name)
		C.free(icon)
		return platform.ApplicationIdentity{}, &platform.ApplicationError{Code: platform.ApplicationNative}
	}
	defer C.free(identifier)
	defer C.free(name)
	defer C.free(icon)

	info := C.dg_application_info_v2{
		struct_size: C.uint32_t(C.sizeof_dg_application_info_v2),
		identifier:  C.dg_application_buffer_v1{data: (*C.uint8_t)(identifier), capacity: applicationIdentifierBufferSize},
		name:        C.dg_application_buffer_v1{data: (*C.uint8_t)(name), capacity: applicationNameBufferSize},
		icon_png:    C.dg_application_buffer_v1{data: (*C.uint8_t)(icon), capacity: applicationIconBufferSize},
	}
	nativeError := C.dg_application_error_v1{struct_size: C.uint32_t(C.sizeof_dg_application_error_v1)}
	status := call(C.uint32_t(C.DG_APPLICATION_ABI_MAJOR), &info, &nativeError)
	if err := ctx.Err(); err != nil {
		return platform.ApplicationIdentity{}, err
	}
	if status != C.DG_APPLICATION_OK {
		return platform.ApplicationIdentity{}, mapApplicationError(status, nativeError)
	}
	if info.identifier.len == 0 || info.identifier.len > applicationIdentifierBufferSize ||
		info.name.len == 0 || info.name.len > applicationNameBufferSize || info.icon_png.len > applicationIconBufferSize {
		return platform.ApplicationIdentity{}, &platform.ApplicationError{Code: platform.ApplicationNative}
	}
	identity := platform.ApplicationIdentity{
		ID:   C.GoStringN((*C.char)(unsafe.Pointer(identifier)), C.int(info.identifier.len)),
		Name: C.GoStringN((*C.char)(unsafe.Pointer(name)), C.int(info.name.len)),
	}
	if info.icon_png.len > 0 {
		identity.IconPNG = C.GoBytes(icon, C.int(info.icon_png.len))
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
