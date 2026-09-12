#ifndef _WIN32_WINNT
#define _WIN32_WINNT 0x0601
#endif

#include "../include/daygo_system.h"
#include "../include/daygo_status_item.h"
#include "Sources/daygo_windows_runtime.h"

#include <atomic>
#include <cstdio>
#include <shellapi.h>
#include <windows.h>
#include <wtsapi32.h>

namespace {
std::atomic<unsigned> event_count{0};
uint32_t observed_events[4]{};
std::atomic<unsigned> action_count{0};
uint32_t observed_actions[3]{};

void on_event(uint32_t kind, int64_t, void*) {
  const unsigned index = event_count.fetch_add(1, std::memory_order_relaxed);
  if (index < 4) {
    observed_events[index] = kind;
  }
}

void on_action(uint32_t action, void*) {
  const unsigned index = action_count.fetch_add(1, std::memory_order_relaxed);
  if (index < 3) {
    observed_actions[index] = action;
  }
}
}  // namespace

int main() {
  const int32_t start = dg_system_start(DG_SYSTEM_ABI_MAJOR, on_event, nullptr);
  if (start != 0) {
    std::fprintf(stderr, "system event ABI start failed: %ld\n", static_cast<long>(start));
    return 1;
  }
  const HWND window = FindWindowW(L"DaygoSystemEventWindowV1", nullptr);
  if (window == nullptr) {
    std::fprintf(stderr, "system event ABI window was not created\n");
    dg_system_stop();
    return 1;
  }

  dg_status_item_state_v1 status{1, 0, "Starting", "Daygo", "Open Daygo",
                                 "Pause Recording", "Quit Daygo"};
  if (const int32_t result = dg_status_item_set(
          DG_STATUS_ITEM_ABI_MAJOR, &status, on_action, nullptr);
      result != 0) {
    std::fprintf(stderr, "status item ABI set failed: %ld\n",
                 static_cast<long>(result));
    dg_system_stop();
    return 1;
  }
  NOTIFYICONIDENTIFIER identifier{};
  identifier.cbSize = sizeof(identifier);
  identifier.hWnd = window;
  identifier.uID = 1;
  RECT icon_rect{};
  if (FAILED(Shell_NotifyIconGetRect(&identifier, &icon_rect))) {
    std::fprintf(stderr, "status item was not registered with the shell\n");
    dg_system_stop();
    return 1;
  }

  // The disabled pause command must be ignored; the remaining commands use
  // the same WM_COMMAND path as TrackPopupMenu.
  SendMessageW(window, WM_COMMAND,
               MAKEWPARAM(kDaygoStatusCommandTogglePause, 0), 0);
  if (action_count.load(std::memory_order_acquire) != 0) {
    std::fprintf(stderr, "disabled pause action was dispatched\n");
    dg_system_stop();
    return 1;
  }
  status.pause_enabled = 1;
  status.title = "Recording";
  if (dg_status_item_set(DG_STATUS_ITEM_ABI_MAJOR, &status, on_action, nullptr) != 0) {
    std::fprintf(stderr, "status item ABI update failed\n");
    dg_system_stop();
    return 1;
  }
  SendMessageW(window, WM_COMMAND, MAKEWPARAM(kDaygoStatusCommandOpen, 0), 0);
  SendMessageW(window, WM_COMMAND,
               MAKEWPARAM(kDaygoStatusCommandTogglePause, 0), 0);
  SendMessageW(window, WM_COMMAND, MAKEWPARAM(kDaygoStatusCommandQuit, 0), 0);
  const uint32_t expected_actions[] = {DG_STATUS_ITEM_OPEN,
                                       DG_STATUS_ITEM_TOGGLE_PAUSE,
                                       DG_STATUS_ITEM_QUIT};
  if (action_count.load(std::memory_order_acquire) != 3) {
    std::fprintf(stderr, "status item ABI observed %u actions, want 3\n",
                 action_count.load());
    dg_system_stop();
    return 1;
  }
  for (unsigned index = 0; index < 3; ++index) {
    if (observed_actions[index] != expected_actions[index]) {
      std::fprintf(stderr, "status action %u = %u, want %u\n", index,
                   observed_actions[index], expected_actions[index]);
      dg_system_stop();
      return 1;
    }
  }
  SendMessageW(window, WM_POWERBROADCAST, PBT_APMSUSPEND, 0);
  SendMessageW(window, WM_POWERBROADCAST, PBT_APMRESUMEAUTOMATIC, 0);
  SendMessageW(window, WM_WTSSESSION_CHANGE, WTS_SESSION_LOCK, 0);
  SendMessageW(window, WM_WTSSESSION_CHANGE, WTS_SESSION_UNLOCK, 0);
  const uint32_t expected[] = {DG_SYSTEM_SLEEP, DG_SYSTEM_WAKE,
                               DG_SYSTEM_SCREEN_LOCKED, DG_SYSTEM_SCREEN_UNLOCKED};
  if (event_count.load(std::memory_order_acquire) != 4) {
    std::fprintf(stderr, "system event ABI observed %u events, want 4\n", event_count.load());
    dg_system_stop();
    return 1;
  }
  for (unsigned index = 0; index < 4; ++index) {
    if (observed_events[index] != expected[index]) {
      std::fprintf(stderr, "system event %u = %u, want %u\n", index,
                   observed_events[index], expected[index]);
      dg_system_stop();
      return 1;
    }
  }
  status.visible = 0;
  if (dg_status_item_set(DG_STATUS_ITEM_ABI_MAJOR, &status, on_action, nullptr) != 0 ||
      SUCCEEDED(Shell_NotifyIconGetRect(&identifier, &icon_rect))) {
    std::fprintf(stderr, "status item ABI hide failed\n");
    dg_system_stop();
    return 1;
  }
  dg_status_item_stop();
  dg_status_item_stop();
  dg_system_stop();
  dg_system_stop();
  std::printf("system/status ABI lifecycle ok; events=%u actions=%u\n",
              event_count.load(), action_count.load());
  return 0;
}
