#ifndef _WIN32_WINNT
#define _WIN32_WINNT 0x0600
#endif

#include "daygo_system.h"
#include "daygo_windows_runtime.h"

#include <windows.h>
#include <wtsapi32.h>

#include <atomic>
#include <chrono>
#include <cstdint>

namespace {

constexpr wchar_t kWindowClassName[] = L"DaygoSystemEventWindowV1";
constexpr UINT kStopMessage = WM_APP + 1;

SRWLOCK g_api_lock = SRWLOCK_INIT;
HANDLE g_thread = nullptr;
HANDLE g_ready = nullptr;
DWORD g_thread_id = 0;
std::atomic<HWND> g_window{nullptr};
std::atomic<int32_t> g_start_result{-1};
std::atomic<dg_system_event_callback_v1> g_callback{nullptr};
std::atomic<void*> g_user_data{nullptr};

class ExclusiveLock {
 public:
  explicit ExclusiveLock(SRWLOCK* lock) : lock_(lock) { AcquireSRWLockExclusive(lock_); }
  ~ExclusiveLock() { ReleaseSRWLockExclusive(lock_); }

  ExclusiveLock(const ExclusiveLock&) = delete;
  ExclusiveLock& operator=(const ExclusiveLock&) = delete;

 private:
  SRWLOCK* lock_;
};

int64_t unix_time_ns() {
  return std::chrono::duration_cast<std::chrono::nanoseconds>(
             std::chrono::system_clock::now().time_since_epoch())
      .count();
}

void emit(uint32_t kind) {
  const auto callback = g_callback.load(std::memory_order_acquire);
  if (callback != nullptr) {
    callback(kind, unix_time_ns(), g_user_data.load(std::memory_order_acquire));
  }
}

LRESULT CALLBACK window_proc(HWND window, UINT message, WPARAM wparam, LPARAM lparam) {
  LRESULT status_result = 0;
  if (daygo_status_item_handle_message(window, message, wparam, lparam,
                                       &status_result)) {
    return status_result;
  }
  switch (message) {
    case WM_POWERBROADCAST:
      if (wparam == PBT_APMSUSPEND) {
        emit(DG_SYSTEM_SLEEP);
      } else if (wparam == PBT_APMRESUMEAUTOMATIC) {
        // RESUMEAUTOMATIC is delivered for every resume. Do not also emit for
        // RESUMESUSPEND, which may immediately follow it after user input.
        emit(DG_SYSTEM_WAKE);
      }
      return TRUE;
    case WM_WTSSESSION_CHANGE:
      if (wparam == WTS_SESSION_LOCK) {
        emit(DG_SYSTEM_SCREEN_LOCKED);
      } else if (wparam == WTS_SESSION_UNLOCK) {
        emit(DG_SYSTEM_SCREEN_UNLOCKED);
      }
      return 0;
    case kStopMessage:
      daygo_status_item_shutdown_on_window_thread(window);
      WTSUnRegisterSessionNotification(window);
      DestroyWindow(window);
      return 0;
    case WM_DESTROY:
      daygo_status_item_shutdown_on_window_thread(window);
      PostQuitMessage(0);
      return 0;
    default:
      return DefWindowProcW(window, message, wparam, lparam);
  }
}

DWORD WINAPI message_thread(void*) {
  const HINSTANCE instance = GetModuleHandleW(nullptr);
  WNDCLASSEXW window_class{};
  window_class.cbSize = sizeof(window_class);
  window_class.lpfnWndProc = window_proc;
  window_class.hInstance = instance;
  window_class.lpszClassName = kWindowClassName;

  if (RegisterClassExW(&window_class) == 0 && GetLastError() != ERROR_CLASS_ALREADY_EXISTS) {
    g_start_result.store(-4, std::memory_order_release);
    SetEvent(g_ready);
    return 0;
  }

  // This is an invisible top-level window. A message-only HWND_MESSAGE window
  // cannot receive broadcast messages such as WM_POWERBROADCAST.
  const HWND window = CreateWindowExW(0, kWindowClassName, L"", WS_POPUP,
                                      0, 0, 0, 0, nullptr, nullptr, instance, nullptr);
  if (window == nullptr) {
    g_start_result.store(-4, std::memory_order_release);
    SetEvent(g_ready);
    return 0;
  }

  if (!WTSRegisterSessionNotification(window, NOTIFY_FOR_THIS_SESSION)) {
    g_start_result.store(-5, std::memory_order_release);
    DestroyWindow(window);
    SetEvent(g_ready);
    return 0;
  }

  g_window.store(window, std::memory_order_release);
  g_start_result.store(0, std::memory_order_release);
  SetEvent(g_ready);

  MSG message{};
  while (GetMessageW(&message, nullptr, 0, 0) > 0) {
    TranslateMessage(&message);
    DispatchMessageW(&message);
  }

  if (IsWindow(window)) {
    WTSUnRegisterSessionNotification(window);
    DestroyWindow(window);
  }
  g_window.store(nullptr, std::memory_order_release);
  return 0;
}

}  // namespace

HWND daygo_windows_system_window() {
  return g_window.load(std::memory_order_acquire);
}

extern "C" int32_t dg_system_start(uint32_t requested_abi_major,
                                     dg_system_event_callback_v1 callback,
                                     void* user_data) {
  if (requested_abi_major != DG_SYSTEM_ABI_MAJOR || callback == nullptr) {
    return -1;
  }

  ExclusiveLock lock(&g_api_lock);
  if (g_thread != nullptr) {
    return -2;
  }

  g_callback.store(callback, std::memory_order_release);
  g_user_data.store(user_data, std::memory_order_release);
  g_start_result.store(-3, std::memory_order_release);
  g_ready = CreateEventW(nullptr, TRUE, FALSE, nullptr);
  if (g_ready == nullptr) {
    g_callback.store(nullptr, std::memory_order_release);
    g_user_data.store(nullptr, std::memory_order_release);
    return -3;
  }
  g_thread = CreateThread(nullptr, 0, message_thread, nullptr, 0, &g_thread_id);
  if (g_thread == nullptr) {
    CloseHandle(g_ready);
    g_ready = nullptr;
    g_callback.store(nullptr, std::memory_order_release);
    g_user_data.store(nullptr, std::memory_order_release);
    return -3;
  }

  WaitForSingleObject(g_ready, INFINITE);
  const int32_t result = g_start_result.load(std::memory_order_acquire);
  CloseHandle(g_ready);
  g_ready = nullptr;
  if (result != 0) {
    PostThreadMessageW(g_thread_id, WM_QUIT, 0, 0);
    WaitForSingleObject(g_thread, INFINITE);
    CloseHandle(g_thread);
    g_thread = nullptr;
    g_thread_id = 0;
    g_callback.store(nullptr, std::memory_order_release);
    g_user_data.store(nullptr, std::memory_order_release);
  }
  return result;
}

extern "C" void dg_system_stop(void) {
  ExclusiveLock lock(&g_api_lock);
  if (g_thread == nullptr) {
    return;
  }

  const HWND window = g_window.load(std::memory_order_acquire);
  if (window != nullptr) {
    PostMessageW(window, kStopMessage, 0, 0);
  } else {
    PostThreadMessageW(g_thread_id, WM_QUIT, 0, 0);
  }
  WaitForSingleObject(g_thread, INFINITE);
  CloseHandle(g_thread);
  g_thread = nullptr;
  g_thread_id = 0;
  g_callback.store(nullptr, std::memory_order_release);
  g_user_data.store(nullptr, std::memory_order_release);
}
