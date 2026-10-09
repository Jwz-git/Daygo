#ifndef DAYGO_NOTIFICATIONS_H
#define DAYGO_NOTIFICATIONS_H

#include <stdint.h>
#include <wchar.h>
#ifdef DAYGO_NOTIFICATIONS_BUILD
#define DG_NOTIFICATION_API __declspec(dllexport)
#else
#define DG_NOTIFICATION_API __declspec(dllimport)
#endif
#ifdef __cplusplus
extern "C" {
#endif

#define DG_NOTIFICATION_ABI_MAJOR 1
enum dg_notification_status {
  DG_NOTIFICATION_OK = 0,
  DG_NOTIFICATION_UNAVAILABLE = 1,
  DG_NOTIFICATION_DENIED = 2,
  DG_NOTIFICATION_INVALID = 3,
  DG_NOTIFICATION_FAILED = 4
};

// UTF-16, NUL-terminated bounded strings; all pointers borrowed for the call.
// A session owns one MTA/COM registration thread. Close joins it; existing OS
// schedules and persistent identity survive close. Never pass a closed handle.
DG_NOTIFICATION_API int32_t dg_notification_open(
    uint32_t abi, const wchar_t* app_id, const wchar_t* activator_clsid,
    const wchar_t* executable, void** session, int32_t* native_error);
DG_NOTIFICATION_API void dg_notification_close(void* session);
// permission: 1 granted, 2 denied (including policy/system-wide restrictions).
DG_NOTIFICATION_API int32_t dg_notification_permission(
    uint32_t abi, void* session, uint32_t* permission, int32_t* native_error);
// delivery_ticks: UTC 100ns ticks since 1601-01-01; 0/past means immediate.
// token: exactly 16 lowercase hex characters. Full ID is checked separately.
DG_NOTIFICATION_API int32_t dg_notification_schedule(
    uint32_t abi, void* session, const wchar_t* id, const wchar_t* token,
    const wchar_t* title, const wchar_t* body, int64_t delivery_ticks,
    int32_t* native_error);
DG_NOTIFICATION_API int32_t dg_notification_cancel(
    uint32_t abi, void* session, const wchar_t* id, const wchar_t* token,
    int32_t* native_error);

#ifdef __cplusplus
}
#endif
#endif
