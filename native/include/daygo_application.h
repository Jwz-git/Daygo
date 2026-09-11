#ifndef DAYGO_APPLICATION_H
#define DAYGO_APPLICATION_H

#include <stddef.h>
#include <stdint.h>

#if UINTPTR_MAX != UINT64_MAX
#error "Daygo application ABI supports 64-bit targets only"
#endif

#if defined(_WIN32)
#define DG_APPLICATION_CALL __cdecl
#if defined(DAYGO_APPLICATION_BUILD)
#define DG_APPLICATION_API __declspec(dllexport)
#elif defined(DAYGO_APPLICATION_STATIC)
#define DG_APPLICATION_API
#else
#define DG_APPLICATION_API __declspec(dllimport)
#endif
#else
#define DG_APPLICATION_CALL
#define DG_APPLICATION_API __attribute__((visibility("default")))
#endif

#if defined(__cplusplus)
extern "C" {
#endif

#define DG_APPLICATION_ABI_MAJOR 1u
#define DG_APPLICATION_ABI_MINOR 0u

typedef struct dg_application_string_view_v1 {
    const uint8_t *data;
    uint64_t len;
} dg_application_string_view_v1;

typedef struct dg_application_buffer_v1 {
    uint8_t *data;
    uint64_t capacity;
    uint64_t len;
} dg_application_buffer_v1;

enum {
    DG_APPLICATION_NATIVE_NONE = 0,
    DG_APPLICATION_NATIVE_POSIX = 1,
    DG_APPLICATION_NATIVE_APPLE = 2
};

enum {
    DG_APPLICATION_OK = 0,
    DG_APPLICATION_E_INVALID_ARGUMENT = -1,
    DG_APPLICATION_E_ABI_MISMATCH = -2,
    DG_APPLICATION_E_UNSUPPORTED = -3,
    DG_APPLICATION_E_NOT_APPLICATION = -4,
    DG_APPLICATION_E_INTERNAL = -5
};

/*
 * Caller-owned output for one selected macOS application bundle.
 *
 * The caller zero-initializes this struct, sets struct_size, and supplies both
 * buffers. On success identifier is the bundle identifier used by Daygo's
 * capture privacy filter, and name is a localized display label. The
 * implementation never retains or allocates memory for the caller.
 */
typedef struct dg_application_info_v1 {
    uint32_t struct_size;
    uint32_t reserved0;
    dg_application_buffer_v1 identifier;
    dg_application_buffer_v1 name;
} dg_application_info_v1;

typedef struct dg_application_error_v1 {
    uint32_t struct_size;
    uint32_t native_domain;
    int64_t native_code;
} dg_application_error_v1;

DG_APPLICATION_API void DG_APPLICATION_CALL dg_application_abi_version(
    uint32_t *major,
    uint32_t *minor
);

/*
 * Inspects an absolute path to a user-selected .app bundle. The bundle must
 * contain a non-empty CFBundleIdentifier. No path, handle, or result is
 * retained after return.
 */
DG_APPLICATION_API int32_t DG_APPLICATION_CALL dg_application_inspect(
    uint32_t requested_abi_major,
    dg_application_string_view_v1 application_path,
    dg_application_info_v1 *out_info,
    dg_application_error_v1 *out_error
);

#if defined(__cplusplus)
} /* extern "C" */
#endif

#if defined(__cplusplus)
#define DG_APPLICATION_STATIC_ASSERT(condition, message) static_assert(condition, message)
#else
#define DG_APPLICATION_STATIC_ASSERT(condition, message) _Static_assert(condition, message)
#endif

DG_APPLICATION_STATIC_ASSERT(
    sizeof(dg_application_string_view_v1) == 16,
    "dg_application_string_view_v1 ABI drift"
);
DG_APPLICATION_STATIC_ASSERT(
    sizeof(dg_application_buffer_v1) == 24,
    "dg_application_buffer_v1 ABI drift"
);
DG_APPLICATION_STATIC_ASSERT(
    sizeof(dg_application_info_v1) == 56,
    "dg_application_info_v1 ABI drift"
);
DG_APPLICATION_STATIC_ASSERT(
    offsetof(dg_application_info_v1, identifier) == 8,
    "dg_application_info_v1.identifier ABI drift"
);
DG_APPLICATION_STATIC_ASSERT(
    offsetof(dg_application_info_v1, name) == 32,
    "dg_application_info_v1.name ABI drift"
);
DG_APPLICATION_STATIC_ASSERT(
    sizeof(dg_application_error_v1) == 16,
    "dg_application_error_v1 ABI drift"
);
DG_APPLICATION_STATIC_ASSERT(
    offsetof(dg_application_error_v1, native_code) == 8,
    "dg_application_error_v1.native_code ABI drift"
);

#undef DG_APPLICATION_STATIC_ASSERT

#endif /* DAYGO_APPLICATION_H */
