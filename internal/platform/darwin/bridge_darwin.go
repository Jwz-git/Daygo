//go:build darwin && cgo

package darwin

/*
#cgo CFLAGS: -I${SRCDIR}/../../../native/include
#cgo LDFLAGS: ${SRCDIR}/../../../build/native/darwin/universal/libdaygo_capture.a
#cgo LDFLAGS: -framework CoreGraphics -framework Foundation -framework ImageIO
#cgo LDFLAGS: -framework ScreenCaptureKit -framework UniformTypeIdentifiers
#cgo LDFLAGS: -framework AVFoundation -framework CoreMedia -framework VideoToolbox
#cgo LDFLAGS: -Wl,-rpath,/usr/lib/swift
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

func frameAppend(ctx context.Context, req platform.CaptureRequest) (platform.CaptureResult, error) {
	var libraryMajor, libraryMinor C.uint32_t
	C.dg_capture_abi_version(&libraryMajor, &libraryMinor)
	if uint32(libraryMajor) != uint32(C.DG_CAPTURE_ABI_MAJOR) {
		return platform.CaptureResult{}, &platform.CaptureError{Code: platform.CaptureABIMismatch}
	}

	dirBytes := []byte(req.SegmentDirectory)
	dirData := C.CBytes(dirBytes)
	if dirData == nil {
		return platform.CaptureResult{}, &platform.CaptureError{Code: platform.CaptureNative}
	}
	defer C.free(dirData)

	blockedViews, releaseBlocked, err := makeBlockedApplicationViews(req.BlockedApplicationIDs)
	if err != nil {
		return platform.CaptureResult{}, err
	}
	defer releaseBlocked()

	flags := C.uint32_t(0)
	if req.ShowsCursor {
		flags |= C.uint32_t(C.DG_CAPTURE_SHOWS_CURSOR)
	}
	request := C.dg_frame_append_request_v1{
		struct_size:                  C.uint32_t(C.sizeof_dg_frame_append_request_v1),
		flags:                        flags,
		target_height:                C.uint32_t(req.TargetHeight),
		timeout_ms:                   C.uint32_t(captureTimeoutMillis(ctx)),
		blocked_application_id_count: C.uint32_t(len(req.BlockedApplicationIDs)),
		synthetic_width:              0,
		synthetic_height:             0,
		reserved0:                    0,
		recordings_dir: C.dg_capture_string_view_v1{
			data: (*C.uint8_t)(dirData),
			len:  C.uint64_t(len(dirBytes)),
		},
		blocked_application_ids: blockedViews,
		reserved1:               0,
	}
	result := C.dg_frame_append_result_v1{struct_size: C.uint32_t(C.sizeof_dg_frame_append_result_v1)}
	nativeError := C.dg_capture_error_v1{struct_size: C.uint32_t(C.sizeof_dg_capture_error_v1)}

	status := C.dg_frame_append(
		C.uint32_t(C.DG_CAPTURE_ABI_MAJOR),
		&request,
		&result,
		&nativeError,
	)
	runtime.KeepAlive(req)

	if ctxErr := ctx.Err(); ctxErr != nil {
		return platform.CaptureResult{}, ctxErr
	}

	switch status {
	case C.DG_CAPTURE_OK, C.DG_CAPTURE_BLOCKED:
		segmentRelPath := C.GoString(&result.segment_rel_path[0])
		outcome := platform.CaptureWritten
		if status == C.DG_CAPTURE_BLOCKED {
			outcome = platform.CaptureBlocked
		}
		return platform.CaptureResult{
			Outcome:     outcome,
			CapturedAt:  time.Unix(0, int64(result.captured_at_unix_ns)),
			Width:       int(result.width),
			Height:      int(result.height),
			FileSize:    int64(result.file_size),
			SegmentPath: segmentRelPath,
			FrameIndex:  int(result.frame_index),
		}, nil
	default:
		return platform.CaptureResult{}, mapCaptureError(status, nativeError)
	}
}

func frameDecode(ctx context.Context, recordingsDir, segmentRelPath string, frameIndex int, maxPixelSize int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dirBytes := []byte(recordingsDir)
	dirData := C.CBytes(dirBytes)
	if dirData == nil {
		return nil, &platform.CaptureError{Code: platform.CaptureNative}
	}
	defer C.free(dirData)

	pathBytes := []byte(segmentRelPath)
	pathData := C.CBytes(pathBytes)
	if pathData == nil {
		return nil, &platform.CaptureError{Code: platform.CaptureNative}
	}
	defer C.free(pathData)

	var outData *C.uint8_t
	var outLen C.uint64_t

	status := C.dg_frame_decode(
		C.dg_capture_string_view_v1{
			data: (*C.uint8_t)(dirData),
			len:  C.uint64_t(len(dirBytes)),
		},
		C.dg_capture_string_view_v1{
			data: (*C.uint8_t)(pathData),
			len:  C.uint64_t(len(pathBytes)),
		},
		C.uint32_t(frameIndex),
		C.uint32_t(maxPixelSize),
		&outData,
		&outLen,
	)
	if status != C.DG_CAPTURE_OK {
		return nil, fmt.Errorf("frameDecode: failed with status %d", int(status))
	}
	if outData == nil || outLen == 0 {
		return nil, fmt.Errorf("frameDecode: empty frame returned")
	}
	defer C.dg_frame_free(outData)

	data := C.GoBytes(unsafe.Pointer(outData), C.int(outLen))
	return data, nil
}

func segmentProbe(ctx context.Context, recordingsDir, segmentRelPath string) (platform.SegmentInfo, error) {
	if err := ctx.Err(); err != nil {
		return platform.SegmentInfo{}, err
	}
	dirBytes := []byte(recordingsDir)
	dirData := C.CBytes(dirBytes)
	if dirData == nil {
		return platform.SegmentInfo{}, &platform.CaptureError{Code: platform.CaptureNative}
	}
	defer C.free(dirData)

	pathBytes := []byte(segmentRelPath)
	pathData := C.CBytes(pathBytes)
	if pathData == nil {
		return platform.SegmentInfo{}, &platform.CaptureError{Code: platform.CaptureNative}
	}
	defer C.free(pathData)

	var info C.dg_segment_info_v1
	info.struct_size = C.uint32_t(C.sizeof_dg_segment_info_v1)

	status := C.dg_segment_probe(
		C.dg_capture_string_view_v1{
			data: (*C.uint8_t)(dirData),
			len:  C.uint64_t(len(dirBytes)),
		},
		C.dg_capture_string_view_v1{
			data: (*C.uint8_t)(pathData),
			len:  C.uint64_t(len(pathBytes)),
		},
		&info,
	)
	if status != C.DG_CAPTURE_OK {
		return platform.SegmentInfo{Readable: false}, nil
	}
	return platform.SegmentInfo{
		FrameCount: int(info.frame_count),
		Width:      int(info.width),
		Height:     int(info.height),
		Readable:   info.readable != 0,
	}, nil
}

func segmentCloseActive() error {
	status := C.dg_segment_close_active()
	if status != C.DG_CAPTURE_OK {
		return fmt.Errorf("segmentCloseActive: failed with status %d", int(status))
	}
	return nil
}

func testFrameAppendSynthetic(recordingsDir string, width, height, frameIndex int) (platform.CaptureResult, error) {
	dirBytes := []byte(recordingsDir)
	dirData := C.CBytes(dirBytes)
	if dirData == nil {
		return platform.CaptureResult{}, &platform.CaptureError{Code: platform.CaptureNative}
	}
	defer C.free(dirData)

	request := C.dg_frame_append_request_v1{
		struct_size:      C.uint32_t(C.sizeof_dg_frame_append_request_v1),
		synthetic_width:  C.uint32_t(width),
		synthetic_height: C.uint32_t(height),
		reserved0:        C.uint32_t(frameIndex),
		recordings_dir: C.dg_capture_string_view_v1{
			data: (*C.uint8_t)(dirData),
			len:  C.uint64_t(len(dirBytes)),
		},
	}
	result := C.dg_frame_append_result_v1{struct_size: C.uint32_t(C.sizeof_dg_frame_append_result_v1)}
	nativeError := C.dg_capture_error_v1{struct_size: C.uint32_t(C.sizeof_dg_capture_error_v1)}

	status := C.dg_frame_append(
		C.uint32_t(C.DG_CAPTURE_ABI_MAJOR),
		&request,
		&result,
		&nativeError,
	)
	if status != C.DG_CAPTURE_OK {
		return platform.CaptureResult{}, mapCaptureError(status, nativeError)
	}

	segmentRelPath := C.GoString(&result.segment_rel_path[0])
	return platform.CaptureResult{
		Outcome:     platform.CaptureWritten,
		CapturedAt:  time.Unix(0, int64(result.captured_at_unix_ns)),
		Width:       int(result.width),
		Height:      int(result.height),
		FileSize:    int64(result.file_size),
		SegmentPath: segmentRelPath,
		FrameIndex:  int(result.frame_index),
	}, nil
}
