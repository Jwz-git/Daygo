//go:build darwin && cgo

package darwin

/*
#cgo CFLAGS: -I${SRCDIR}/../../../native/include
#cgo LDFLAGS: ${SRCDIR}/../../../build/native/darwin/universal/libdaygo_capture.a
#cgo LDFLAGS: -framework CoreGraphics -framework Foundation -framework ImageIO
#cgo LDFLAGS: -framework ScreenCaptureKit -framework Security -framework UniformTypeIdentifiers
#cgo LDFLAGS: -Wl,-rpath,/usr/lib/swift
#include <stdlib.h>
#include "daygo_capture.h"
*/
import "C"

import (
	"context"
	"os"
	"runtime"
	"time"
	"unsafe"

	"github.com/Jwz-git/Daygo/internal/platform"
)

const defaultCaptureTimeout = 10 * time.Second

func captureOnce(ctx context.Context, req platform.CaptureRequest) (platform.CaptureResult, error) {
	var libraryMajor, libraryMinor C.uint32_t
	C.dg_capture_abi_version(&libraryMajor, &libraryMinor)
	if uint32(libraryMajor) != uint32(C.DG_CAPTURE_ABI_MAJOR) {
		return platform.CaptureResult{}, &platform.CaptureError{Code: platform.CaptureABIMismatch}
	}

	outputBytes := []byte(req.OutputPath)
	outputData := C.CBytes(outputBytes)
	if outputData == nil {
		return platform.CaptureResult{}, &platform.CaptureError{Code: platform.CaptureNative}
	}
	defer C.free(outputData)

	blockedViews, releaseBlocked, err := makeBlockedApplicationViews(req.BlockedApplicationIDs)
	if err != nil {
		return platform.CaptureResult{}, err
	}
	defer releaseBlocked()

	flags := C.uint32_t(0)
	if req.ShowsCursor {
		flags |= C.uint32_t(C.DG_CAPTURE_SHOWS_CURSOR)
	}
	request := C.dg_capture_request_v1{
		struct_size:                  C.uint32_t(C.sizeof_dg_capture_request_v1),
		flags:                        flags,
		image_format:                 C.uint32_t(C.DG_CAPTURE_IMAGE_JPEG),
		target_height:                C.uint32_t(req.TargetHeight),
		jpeg_quality:                 C.uint32_t(req.JPEGQuality),
		timeout_ms:                   C.uint32_t(captureTimeoutMillis(ctx)),
		blocked_application_id_count: C.uint32_t(len(req.BlockedApplicationIDs)),
		reserved0:                    0,
		output_path: C.dg_capture_string_view_v1{
			data: (*C.uint8_t)(outputData),
			len:  C.uint64_t(len(outputBytes)),
		},
		blocked_application_ids: blockedViews,
	}
	result := C.dg_capture_result_v1{struct_size: C.uint32_t(C.sizeof_dg_capture_result_v1)}
	nativeError := C.dg_capture_error_v1{struct_size: C.uint32_t(C.sizeof_dg_capture_error_v1)}

	status := C.dg_capture_once(
		C.uint32_t(C.DG_CAPTURE_ABI_MAJOR),
		&request,
		&result,
		&nativeError,
	)
	runtime.KeepAlive(req)

	if ctxErr := ctx.Err(); ctxErr != nil {
		if status == C.DG_CAPTURE_OK {
			_ = os.Remove(req.OutputPath)
		}
		return platform.CaptureResult{}, ctxErr
	}

	switch status {
	case C.DG_CAPTURE_OK:
		if result.image_format != C.DG_CAPTURE_IMAGE_JPEG || result.width == 0 || result.height == 0 || result.file_size == 0 {
			_ = os.Remove(req.OutputPath)
			return platform.CaptureResult{}, &platform.CaptureError{Code: platform.CaptureNative}
		}
		return platform.CaptureResult{
			Outcome:    platform.CaptureWritten,
			CapturedAt: time.Unix(0, int64(result.captured_at_unix_ns)),
			Width:      int(result.width),
			Height:     int(result.height),
			FileSize:   int64(result.file_size),
		}, nil
	case C.DG_CAPTURE_BLOCKED:
		return platform.CaptureResult{Outcome: platform.CaptureBlocked}, nil
	default:
		return platform.CaptureResult{}, mapCaptureError(status, nativeError)
	}
}

func makeBlockedApplicationViews(ids []string) (*C.dg_capture_string_view_v1, func(), error) {
	if len(ids) == 0 {
		return nil, func() {}, nil
	}
	allocation := C.calloc(C.size_t(len(ids)), C.size_t(C.sizeof_dg_capture_string_view_v1))
	if allocation == nil {
		return nil, nil, &platform.CaptureError{Code: platform.CaptureNative}
	}
	views := unsafe.Slice((*C.dg_capture_string_view_v1)(allocation), len(ids))
	data := make([]unsafe.Pointer, len(ids))
	for index, id := range ids {
		bytes := []byte(id)
		data[index] = C.CBytes(bytes)
		if data[index] == nil {
			for _, pointer := range data[:index] {
				C.free(pointer)
			}
			C.free(allocation)
			return nil, nil, &platform.CaptureError{Code: platform.CaptureNative}
		}
		views[index] = C.dg_capture_string_view_v1{
			data: (*C.uint8_t)(data[index]),
			len:  C.uint64_t(len(bytes)),
		}
	}
	return (*C.dg_capture_string_view_v1)(allocation), func() {
		for _, pointer := range data {
			C.free(pointer)
		}
		C.free(allocation)
	}, nil
}

func captureTimeoutMillis(ctx context.Context) int64 {
	timeout := defaultCaptureTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	if timeout <= 0 {
		return 1
	}
	if timeout > time.Minute {
		timeout = time.Minute
	}
	return max(1, (timeout.Nanoseconds()+int64(time.Millisecond)-1)/int64(time.Millisecond))
}

func mapCaptureError(status C.int32_t, nativeError C.dg_capture_error_v1) error {
	code := platform.CaptureNative
	switch status {
	case C.DG_CAPTURE_E_INVALID_ARGUMENT:
		code = platform.CaptureInvalidArgument
	case C.DG_CAPTURE_E_ABI_MISMATCH:
		code = platform.CaptureABIMismatch
	case C.DG_CAPTURE_E_UNSUPPORTED:
		code = platform.CaptureUnsupported
	case C.DG_CAPTURE_E_PERMISSION_DENIED:
		code = platform.CapturePermissionDenied
	case C.DG_CAPTURE_E_NO_DISPLAY:
		code = platform.CaptureNoDisplay
	case C.DG_CAPTURE_E_TIMEOUT:
		code = platform.CaptureTimeout
	case C.DG_CAPTURE_E_IO:
		code = platform.CaptureIO
	case C.DG_CAPTURE_E_PRIVACY_UNSUPPORTED:
		code = platform.CapturePrivacyUnsupported
	}
	return &platform.CaptureError{Code: code, NativeCode: int64(nativeError.native_code)}
}
