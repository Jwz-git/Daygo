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
	"unsafe"

	"github.com/Jwz-git/Daygo/internal/platform"
)

const applicationOutputBufferSize = 4096

func inspectApplication(ctx context.Context, path string) (platform.AppInfo, error) {
	var libraryMajor, libraryMinor C.uint32_t
	C.dg_application_abi_version(&libraryMajor, &libraryMinor)
	if uint32(libraryMajor) != uint32(C.DG_APPLICATION_ABI_MAJOR) {
		return platform.AppInfo{}, &platform.ApplicationError{Code: platform.ApplicationABIMismatch}
	}

	pathData := C.CBytes([]byte(path))
	if pathData == nil {
		return platform.AppInfo{}, &platform.ApplicationError{Code: platform.ApplicationNative}
	}
	defer C.free(pathData)

	identifierData := C.malloc(applicationOutputBufferSize)
	nameData := C.malloc(applicationOutputBufferSize)
	if identifierData == nil || nameData == nil {
		C.free(identifierData)
		C.free(nameData)
		return platform.AppInfo{}, &platform.ApplicationError{Code: platform.ApplicationNative}
	}
	defer C.free(identifierData)
	defer C.free(nameData)

	info := C.dg_application_info_v1{
		struct_size: C.uint32_t(C.sizeof_dg_application_info_v1),
		identifier: C.dg_application_buffer_v1{
			data:     (*C.uint8_t)(identifierData),
			capacity: applicationOutputBufferSize,
		},
		name: C.dg_application_buffer_v1{
			data:     (*C.uint8_t)(nameData),
			capacity: applicationOutputBufferSize,
		},
	}
	nativeError := C.dg_application_error_v1{struct_size: C.uint32_t(C.sizeof_dg_application_error_v1)}
	status := C.dg_application_inspect(
		C.uint32_t(C.DG_APPLICATION_ABI_MAJOR),
		C.dg_application_string_view_v1{
			data: (*C.uint8_t)(pathData),
			len:  C.uint64_t(len(path)),
		},
		&info,
		&nativeError,
	)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return platform.AppInfo{}, ctxErr
	}
	if status != C.DG_APPLICATION_OK {
		return platform.AppInfo{}, mapApplicationError(status, nativeError)
	}
	if info.identifier.len == 0 || info.identifier.len > applicationOutputBufferSize ||
		info.name.len == 0 || info.name.len > applicationOutputBufferSize {
		return platform.AppInfo{}, &platform.ApplicationError{Code: platform.ApplicationNative}
	}

	return platform.AppInfo{
		ID:   C.GoStringN((*C.char)(unsafe.Pointer(identifierData)), C.int(info.identifier.len)),
		Name: C.GoStringN((*C.char)(unsafe.Pointer(nameData)), C.int(info.name.len)),
	}, nil
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

	}
	return &platform.ApplicationError{Code: code, NativeCode: int64(nativeError.native_code)}
}
