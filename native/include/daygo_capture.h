#ifndef DAYGO_CAPTURE_H
#define DAYGO_CAPTURE_H

#include <stddef.h>
#include <stdint.h>

#if UINTPTR_MAX != UINT64_MAX
#error "Daygo capture ABI supports 64-bit targets only"
#endif

#if defined(_WIN32)
#define DG_CAPTURE_CALL __cdecl
#if defined(DAYGO_CAPTURE_BUILD)
#define DG_CAPTURE_API __declspec(dllexport)
#elif defined(DAYGO_CAPTURE_STATIC)
#define DG_CAPTURE_API
#else
#define DG_CAPTURE_API __declspec(dllimport)
#endif
#else
#define DG_CAPTURE_CALL
#define DG_CAPTURE_API __attribute__((visibility("default")))
#endif

#if defined(__cplusplus)
extern "C" {
#endif

#define DG_CAPTURE_ABI_MAJOR 1u
#define DG_CAPTURE_ABI_MINOR 0u

/*
 * Borrowed UTF-8 bytes. Strings are not NUL-terminated unless the bytes happen
 * to contain a trailing NUL. The callee must not retain data after the call.
 */
typedef struct dg_capture_string_view_v1 {
    const uint8_t *data;
    uint64_t len;
} dg_capture_string_view_v1;

/* dg_capture_request_v1.flags */
enum {
    DG_CAPTURE_SHOWS_CURSOR = 1u << 0
};

/* dg_capture_request_v1.image_format and dg_capture_result_v1.image_format */
enum {
    DG_CAPTURE_IMAGE_JPEG = 1
};

/* dg_capture_error_v1.native_domain */
enum {
    DG_CAPTURE_NATIVE_NONE = 0,
    DG_CAPTURE_NATIVE_POSIX = 1,
    DG_CAPTURE_NATIVE_APPLE = 2,
    DG_CAPTURE_NATIVE_WINDOWS = 3
};

/*
 * Function results. Zero is a written image, positive values are successful
 * control outcomes, and negative values are errors.
 */
enum {
    DG_CAPTURE_OK = 0,
    DG_CAPTURE_BLOCKED = 1,

    DG_CAPTURE_E_INVALID_ARGUMENT = -1,
    DG_CAPTURE_E_ABI_MISMATCH = -2,
    DG_CAPTURE_E_UNSUPPORTED = -3,
    DG_CAPTURE_E_PERMISSION_DENIED = -4,
    DG_CAPTURE_E_NO_DISPLAY = -5,
    DG_CAPTURE_E_TIMEOUT = -6,
    DG_CAPTURE_E_IO = -7,
    DG_CAPTURE_E_PRIVACY_UNSUPPORTED = -8,
    DG_CAPTURE_E_INTERNAL = -9
};

/*
 * Complete input for one screenshot attempt.
 *
 * Every call captures the platform's current primary display. The caller does
 * not enumerate, select, or pass a display identifier.
 *
 * The caller zero-initializes this struct, sets struct_size, and keeps every
 * referenced byte alive until dg_capture_once() returns. The implementation
 * must not retain any pointer. All reserved fields must be zero.
 *
 * output_path:
 *   Absolute UTF-8 path for the final JPEG. Its parent exists and the final
 *   path does not. The implementation writes a sibling temporary file, closes
 *   it, and atomically publishes without replacing an existing file.
 *
 * blocked_application_ids:
 *   Complete per-call privacy snapshot. If the frontmost application matches,
 *   no screenshot API is invoked and DG_CAPTURE_BLOCKED is returned. Otherwise
 *   every listed application must be excluded from the captured image; a
 *   platform unable to guarantee that returns DG_CAPTURE_E_PRIVACY_UNSUPPORTED.
 */
typedef struct dg_capture_request_v1 {
    uint32_t struct_size;
    uint32_t flags;
    uint32_t image_format;
    uint32_t target_height;
    uint32_t jpeg_quality;
    uint32_t timeout_ms;
    uint32_t blocked_application_id_count;
    uint32_t reserved0;

    dg_capture_string_view_v1 output_path;
    const dg_capture_string_view_v1 *blocked_application_ids;
} dg_capture_request_v1;

/*
 * Output for DG_CAPTURE_OK only.
 *
 * The caller zero-initializes this struct and sets struct_size. For every other
 * return code the implementation leaves all fields except struct_size as zero.
 * captured_at_unix_ns is the OS capture time when available; otherwise it is
 * the midpoint between starting and completing the platform screenshot call.
 */
typedef struct dg_capture_result_v1 {
    uint32_t struct_size;
    uint32_t image_format;
    int64_t captured_at_unix_ns;
    uint64_t file_size;
    uint32_t width;
    uint32_t height;
} dg_capture_result_v1;

/*
 * Optional numeric diagnostics for an error result.
 *
 * Business logic must branch on the function result, never native_domain or
 * native_code. The caller zero-initializes this struct and sets struct_size.
 * DG_CAPTURE_OK and DG_CAPTURE_BLOCKED leave both diagnostic fields zero.
 */
typedef struct dg_capture_error_v1 {
    uint32_t struct_size;
    uint32_t native_domain;
    int64_t native_code;
} dg_capture_error_v1;

/* Reports the linked library ABI. Either output pointer may be NULL. */
DG_CAPTURE_API void DG_CAPTURE_CALL dg_capture_abi_version(
    uint32_t *major,
    uint32_t *minor
);

/*
 * Synchronously attempts exactly one screenshot of the current primary display
 * and returns only after the output file is atomically published, the attempt
 * is blocked, or the attempt fails. requested_abi_major must equal
 * DG_CAPTURE_ABI_MAJOR.
 *
 * request and out_result are required. out_error is optional. This function
 * creates no persistent handle, timer, worker, event queue, segment, callback,
 * or recorder state. It never opens SQLite and never reads product settings.
 */
DG_CAPTURE_API int32_t DG_CAPTURE_CALL dg_capture_once(
    uint32_t requested_abi_major,
    const dg_capture_request_v1 *request,
    dg_capture_result_v1 *out_result,
    dg_capture_error_v1 *out_error
);

#if defined(__cplusplus)
} /* extern "C" */
#endif

/*
 * ABI v1 layout. Keep enums out of structs and append fields only at the end of
 * a struct within the same major version.
 */
#if defined(__cplusplus)
#define DG_CAPTURE_STATIC_ASSERT(condition, message) static_assert(condition, message)
#else
#define DG_CAPTURE_STATIC_ASSERT(condition, message) _Static_assert(condition, message)
#endif

DG_CAPTURE_STATIC_ASSERT(
    sizeof(dg_capture_string_view_v1) == 16,
    "dg_capture_string_view_v1 ABI drift"
);
DG_CAPTURE_STATIC_ASSERT(
    sizeof(dg_capture_request_v1) == 56,
    "dg_capture_request_v1 ABI drift"
);
DG_CAPTURE_STATIC_ASSERT(
    offsetof(dg_capture_request_v1, output_path) == 32,
    "dg_capture_request_v1.output_path ABI drift"
);
DG_CAPTURE_STATIC_ASSERT(
    offsetof(dg_capture_request_v1, blocked_application_ids) == 48,
    "dg_capture_request_v1.blocked_application_ids ABI drift"
);
DG_CAPTURE_STATIC_ASSERT(
    sizeof(dg_capture_result_v1) == 32,
    "dg_capture_result_v1 ABI drift"
);
DG_CAPTURE_STATIC_ASSERT(
    offsetof(dg_capture_result_v1, captured_at_unix_ns) == 8,
    "dg_capture_result_v1.captured_at_unix_ns ABI drift"
);
DG_CAPTURE_STATIC_ASSERT(
    sizeof(dg_capture_error_v1) == 16,
    "dg_capture_error_v1 ABI drift"
);
DG_CAPTURE_STATIC_ASSERT(
    offsetof(dg_capture_error_v1, native_code) == 8,
    "dg_capture_error_v1.native_code ABI drift"
);

#undef DG_CAPTURE_STATIC_ASSERT

#endif /* DAYGO_CAPTURE_H */
