//go:build windows && cgo

package windows

/*
#cgo CFLAGS: -DDAYGO_CAPTURE_STATIC -I${SRCDIR}/../../../native/include
#cgo LDFLAGS: ${SRCDIR}/../../../build/native/windows/amd64/libdaygo_capture.a
#cgo LDFLAGS: -ld3d11 -ldxgi -ldxguid -lole32 -loleaut32 -lwindowscodecs -luser32 -lgdi32 -ladvapi32 -lmfplat -lmfreadwrite -lmfuuid -lstdc++ -lgcc -lgcc_eh
#include <stdlib.h>
#include "daygo_capture.h"
*/
import "C"

import (
	"context"
	"fmt"
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
		output_path: C.dg_capture_string_view_v1{
			data: (*C.uint8_t)(outputData),
			len:  C.uint64_t(len(outputBytes)),
		},
		blocked_application_ids: blockedViews,
	}
	result := C.dg_capture_result_v1{struct_size: C.uint32_t(C.sizeof_dg_capture_result_v1)}
	nativeError := C.dg_capture_error_v1{struct_size: C.uint32_t(C.sizeof_dg_capture_error_v1)}
	status := C.dg_capture_once(C.uint32_t(C.DG_CAPTURE_ABI_MAJOR), &request, &result, &nativeError)
	runtime.KeepAlive(req)

	if ctxErr := ctx.Err(); ctxErr != nil {
		if status == C.DG_CAPTURE_OK {
			_ = os.Remove(req.OutputPath)
		}
		return platform.CaptureResult{}, ctxErr
	}
	if status == C.DG_CAPTURE_OK {
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
	}
	if status == C.DG_CAPTURE_BLOCKED {
		return platform.CaptureResult{Outcome: platform.CaptureBlocked}, nil
	}
	return platform.CaptureResult{}, mapCaptureError(status, nativeError)
}

func frameAppend(ctx context.Context, req platform.CaptureRequest) (platform.CaptureResult, error) {
	dir := C.CBytes([]byte(req.SegmentDirectory))
	if dir == nil {
		return platform.CaptureResult{}, &platform.CaptureError{Code: platform.CaptureNative}
	}
	defer C.free(dir)
	blocked, release, err := makeBlockedApplicationViews(req.BlockedApplicationIDs)
	if err != nil {
		return platform.CaptureResult{}, err
	}
	defer release()
	var flags C.uint32_t
	if req.ShowsCursor {
		flags = C.DG_CAPTURE_SHOWS_CURSOR
	}
	nativeReq := C.dg_frame_append_request_v1{
		struct_size: C.sizeof_dg_frame_append_request_v1, flags: flags,
		target_height: C.uint32_t(req.TargetHeight), timeout_ms: C.uint32_t(captureTimeoutMillis(ctx)),
		blocked_application_id_count: C.uint32_t(len(req.BlockedApplicationIDs)),
		recordings_dir:               C.dg_capture_string_view_v1{data: (*C.uint8_t)(dir), len: C.uint64_t(len(req.SegmentDirectory))},
		blocked_application_ids:      blocked,
	}
	result := C.dg_frame_append_result_v1{struct_size: C.sizeof_dg_frame_append_result_v1}
	nativeErr := C.dg_capture_error_v1{struct_size: C.sizeof_dg_capture_error_v1}
	status := C.dg_frame_append(C.DG_CAPTURE_ABI_MAJOR, &nativeReq, &result, &nativeErr)
	runtime.KeepAlive(req)
	if err := ctx.Err(); err != nil {
		return platform.CaptureResult{}, err
	}
	if status != C.DG_CAPTURE_OK && status != C.DG_CAPTURE_BLOCKED {
		return platform.CaptureResult{}, mapCaptureError(status, nativeErr)
	}
	outcome := platform.CaptureWritten
	if status == C.DG_CAPTURE_BLOCKED {
		outcome = platform.CaptureBlocked
	}
	return platform.CaptureResult{Outcome: outcome, CapturedAt: time.Unix(0, int64(result.captured_at_unix_ns)), Width: int(result.width), Height: int(result.height), FileSize: int64(result.file_size), SegmentPath: C.GoString(&result.segment_rel_path[0]), FrameIndex: int(result.frame_index)}, nil
}

func frameDecode(ctx context.Context, root, rel string, frameIndex, maxPixelSize int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rootData, pathData := C.CBytes([]byte(root)), C.CBytes([]byte(rel))
	if rootData == nil || pathData == nil {
		C.free(rootData)
		C.free(pathData)
		return nil, &platform.CaptureError{Code: platform.CaptureNative}
	}
	defer C.free(rootData)
	defer C.free(pathData)
	var data *C.uint8_t
	var size C.uint64_t
	status := C.dg_frame_decode(C.dg_capture_string_view_v1{data: (*C.uint8_t)(rootData), len: C.uint64_t(len(root))}, C.dg_capture_string_view_v1{data: (*C.uint8_t)(pathData), len: C.uint64_t(len(rel))}, C.uint32_t(frameIndex), C.uint32_t(maxPixelSize), &data, &size)
	if status != C.DG_CAPTURE_OK || data == nil || size == 0 {
		return nil, fmt.Errorf("windows frame decode failed: %d", int(status))
	}
	defer C.dg_frame_free(data)
	return C.GoBytes(unsafe.Pointer(data), C.int(size)), nil
}

func segmentProbe(ctx context.Context, root, rel string) (platform.SegmentInfo, error) {
	if err := ctx.Err(); err != nil {
		return platform.SegmentInfo{}, err
	}
	rootData, pathData := C.CBytes([]byte(root)), C.CBytes([]byte(rel))
	if rootData == nil || pathData == nil {
		C.free(rootData)
		C.free(pathData)
		return platform.SegmentInfo{}, &platform.CaptureError{Code: platform.CaptureNative}
	}
	defer C.free(rootData)
	defer C.free(pathData)
	info := C.dg_segment_info_v1{struct_size: C.sizeof_dg_segment_info_v1}
	status := C.dg_segment_probe(C.dg_capture_string_view_v1{data: (*C.uint8_t)(rootData), len: C.uint64_t(len(root))}, C.dg_capture_string_view_v1{data: (*C.uint8_t)(pathData), len: C.uint64_t(len(rel))}, &info)
	if status != C.DG_CAPTURE_OK {
		return platform.SegmentInfo{Readable: false}, nil
	}
	return platform.SegmentInfo{FrameCount: int(info.frame_count), Width: int(info.width), Height: int(info.height), Readable: info.readable != 0}, nil
}

func segmentCloseActive() error {
	if status := C.dg_segment_close_active(); status != C.DG_CAPTURE_OK {
		return fmt.Errorf("windows segment close failed: %d", int(status))
	}
	return nil
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
		views[index] = C.dg_capture_string_view_v1{data: (*C.uint8_t)(data[index]), len: C.uint64_t(len(bytes))}
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
