#ifndef DAYGO_WINDOWS_RUNTIME_H
#define DAYGO_WINDOWS_RUNTIME_H

#include <windows.h>

// Private messages shared by the Windows System-port implementations. They
// are intentionally outside the public C ABI.
constexpr UINT kDaygoStatusApplyMessage = WM_APP + 2;
constexpr UINT kDaygoStatusClearMessage = WM_APP + 3;
constexpr UINT kDaygoStatusCallbackMessage = WM_APP + 4;

constexpr UINT kDaygoStatusCommandOpen = 1001;
constexpr UINT kDaygoStatusCommandTogglePause = 1002;
constexpr UINT kDaygoStatusCommandQuit = 1003;
constexpr UINT kDaygoStatusCommandOpenRecordings = 1004;
constexpr UINT kDaygoStatusCommandPauseIndefinite = 1006;
constexpr UINT kDaygoStatusCommandPause15 = 1007;
constexpr UINT kDaygoStatusCommandPause30 = 1008;
constexpr UINT kDaygoStatusCommandPause60 = 1009;

HWND daygo_windows_system_window();

// Returns true when the status-item adapter consumed the message.
bool daygo_status_item_handle_message(HWND window, UINT message, WPARAM wparam,
                                      LPARAM lparam, LRESULT* result);
void daygo_status_item_shutdown_on_window_thread(HWND window);

#endif
