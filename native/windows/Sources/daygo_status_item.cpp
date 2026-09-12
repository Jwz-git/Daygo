#ifndef _WIN32_WINNT
#define _WIN32_WINNT 0x0600
#endif

#include "daygo_status_item.h"
#include "daygo_windows_runtime.h"

#include <shellapi.h>
#include <windowsx.h>

#include <algorithm>
#include <cstdint>
#include <cwchar>
#include <iterator>
#include <string>
#include <utility>

namespace {

constexpr UINT kStatusIconID = 1;

struct StatusSnapshot {
  bool visible = false;
  bool pause_enabled = false;
  std::wstring title;
  std::wstring tooltip;
  std::wstring open_label;
  std::wstring pause_label;
  std::wstring quit_label;
  dg_status_item_action_callback_v1 callback = nullptr;
  void* user_data = nullptr;
};

struct StatusRuntime {
  bool added = false;
  bool owns_icon = false;
  HICON icon = nullptr;
  UINT taskbar_created = 0;
  StatusSnapshot snapshot;
};

StatusRuntime g_status;

bool utf8_to_wide(const char* value, std::wstring* output) {
  if (value == nullptr || output == nullptr) {
    return false;
  }
  if (*value == '\0') {
    output->clear();
    return true;
  }
  const int length = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, value, -1,
                                         nullptr, 0);
  if (length <= 0) {
    return false;
  }
  std::wstring converted(static_cast<size_t>(length), L'\0');
  if (MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, value, -1,
                          converted.data(), length) <= 0) {
    return false;
  }
  converted.resize(static_cast<size_t>(length - 1));
  *output = std::move(converted);
  return true;
}

std::wstring tooltip_for(const StatusSnapshot& snapshot) {
  if (snapshot.title.empty()) {
    return snapshot.tooltip;
  }
  if (snapshot.tooltip.empty() || snapshot.tooltip == snapshot.title) {
    return snapshot.title;
  }
  return snapshot.tooltip + L" - " + snapshot.title;
}

void copy_tooltip(wchar_t (&destination)[128], const std::wstring& source) {
  const size_t count = (std::min)(source.size(), std::size(destination) - 1);
  std::wmemcpy(destination, source.data(), count);
  destination[count] = L'\0';
}

HICON load_application_icon(bool* owns_icon) {
  *owns_icon = false;
  wchar_t executable[MAX_PATH]{};
  if (GetModuleFileNameW(nullptr, executable, static_cast<DWORD>(std::size(executable))) != 0) {
    HICON large = nullptr;
    HICON small = nullptr;
    if (ExtractIconExW(executable, 0, &large, &small, 1) > 0) {
      if (small != nullptr) {
        if (large != nullptr) {
          DestroyIcon(large);
        }
        *owns_icon = true;
        return small;
      }
      if (large != nullptr) {
        *owns_icon = true;
        return large;
      }
    }
  }
  return LoadIconW(nullptr, MAKEINTRESOURCEW(32512));
}

NOTIFYICONDATAW notification_data(HWND window) {
  NOTIFYICONDATAW data{};
  data.cbSize = sizeof(data);
  data.hWnd = window;
  data.uID = kStatusIconID;
  data.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP;
  data.uCallbackMessage = kDaygoStatusCallbackMessage;
  data.hIcon = g_status.icon;
  copy_tooltip(data.szTip, tooltip_for(g_status.snapshot));
  return data;
}

int32_t add_icon(HWND window) {
  if (g_status.icon == nullptr) {
    g_status.icon = load_application_icon(&g_status.owns_icon);
  }
  NOTIFYICONDATAW data = notification_data(window);
  if (!Shell_NotifyIconW(NIM_ADD, &data)) {
    return -4;
  }
  data.uVersion = NOTIFYICON_VERSION_4;
  if (!Shell_NotifyIconW(NIM_SETVERSION, &data)) {
    Shell_NotifyIconW(NIM_DELETE, &data);
    return -4;
  }
  g_status.added = true;
  return 0;
}

void delete_icon(HWND window) {
  if (!g_status.added) {
    return;
  }
  NOTIFYICONDATAW data = notification_data(window);
  Shell_NotifyIconW(NIM_DELETE, &data);
  g_status.added = false;
}

void release_icon() {
  if (g_status.owns_icon && g_status.icon != nullptr) {
    DestroyIcon(g_status.icon);
  }
  g_status.icon = nullptr;
  g_status.owns_icon = false;
}

int32_t apply_snapshot(HWND window, const StatusSnapshot& snapshot) {
  g_status.snapshot = snapshot;
  if (g_status.taskbar_created == 0) {
    g_status.taskbar_created = RegisterWindowMessageW(L"TaskbarCreated");
  }
  if (!snapshot.visible) {
    delete_icon(window);
    return 0;
  }
  if (!g_status.added) {
    return add_icon(window);
  }
  NOTIFYICONDATAW data = notification_data(window);
  return Shell_NotifyIconW(NIM_MODIFY, &data) ? 0 : -4;
}

void insert_menu_item(HMENU menu, UINT position, UINT command,
                      const std::wstring& label, bool enabled) {
  MENUITEMINFOW item{};
  item.cbSize = sizeof(item);
  item.fMask = MIIM_ID | MIIM_STRING | MIIM_STATE;
  item.wID = command;
  item.fState = enabled ? MFS_ENABLED : MFS_DISABLED;
  item.dwTypeData = const_cast<wchar_t*>(label.c_str());
  InsertMenuItemW(menu, position, TRUE, &item);
}

void emit_action(uint32_t action) {
  const auto callback = g_status.snapshot.callback;
  void* const user_data = g_status.snapshot.user_data;
  if (callback != nullptr) {
    callback(action, user_data);
  }
}

void show_context_menu(HWND window, WPARAM wparam) {
  HMENU menu = CreatePopupMenu();
  if (menu == nullptr) {
    return;
  }
  insert_menu_item(menu, 0, kDaygoStatusCommandOpen,
                   g_status.snapshot.open_label, true);
  insert_menu_item(menu, 1, kDaygoStatusCommandTogglePause,
                   g_status.snapshot.pause_label,
                   g_status.snapshot.pause_enabled);
  MENUITEMINFOW separator{};
  separator.cbSize = sizeof(separator);
  separator.fMask = MIIM_FTYPE;
  separator.fType = MFT_SEPARATOR;
  InsertMenuItemW(menu, 2, TRUE, &separator);
  insert_menu_item(menu, 3, kDaygoStatusCommandQuit,
                   g_status.snapshot.quit_label, true);

  POINT point{GET_X_LPARAM(wparam), GET_Y_LPARAM(wparam)};
  if (point.x == -1 && point.y == -1) {
    GetCursorPos(&point);
  }
  SetForegroundWindow(window);
  const UINT command = TrackPopupMenu(
      menu, TPM_RETURNCMD | TPM_RIGHTBUTTON | TPM_BOTTOMALIGN,
      point.x, point.y, 0, window, nullptr);
  DestroyMenu(menu);
  PostMessageW(window, WM_NULL, 0, 0);

  if (command != 0) {
    SendMessageW(window, WM_COMMAND, MAKEWPARAM(command, 0), 0);
  }
  NOTIFYICONDATAW data = notification_data(window);
  Shell_NotifyIconW(NIM_SETFOCUS, &data);
}

}  // namespace

bool daygo_status_item_handle_message(HWND window, UINT message, WPARAM wparam,
                                      LPARAM lparam, LRESULT* result) {
  if (message == kDaygoStatusApplyMessage) {
    *result = apply_snapshot(window, *reinterpret_cast<const StatusSnapshot*>(lparam));
    return true;
  }
  if (message == kDaygoStatusClearMessage) {
    delete_icon(window);
    release_icon();
    g_status.snapshot = StatusSnapshot{};
    *result = 0;
    return true;
  }
  if (message == kDaygoStatusCallbackMessage &&
      LOWORD(lparam) == WM_CONTEXTMENU) {
    show_context_menu(window, wparam);
    *result = 0;
    return true;
  }
  if (message == WM_COMMAND) {
    switch (LOWORD(wparam)) {
      case kDaygoStatusCommandOpen:
        emit_action(DG_STATUS_ITEM_OPEN);
        *result = 0;
        return true;
      case kDaygoStatusCommandTogglePause:
        if (g_status.snapshot.pause_enabled) {
          emit_action(DG_STATUS_ITEM_TOGGLE_PAUSE);
        }
        *result = 0;
        return true;
      case kDaygoStatusCommandQuit:
        emit_action(DG_STATUS_ITEM_QUIT);
        *result = 0;
        return true;
    }
  }
  if (g_status.taskbar_created != 0 && message == g_status.taskbar_created) {
    g_status.added = false;
    release_icon();
    if (g_status.snapshot.visible) {
      *result = add_icon(window);
    } else {
      *result = 0;
    }
    return true;
  }
  return false;
}

void daygo_status_item_shutdown_on_window_thread(HWND window) {
  delete_icon(window);
  release_icon();
  g_status.snapshot = StatusSnapshot{};
}

extern "C" int32_t dg_status_item_set(
    uint32_t requested_abi_major, const dg_status_item_state_v1* state,
    dg_status_item_action_callback_v1 callback, void* user_data) {
  if (requested_abi_major != DG_STATUS_ITEM_ABI_MAJOR || state == nullptr ||
      callback == nullptr) {
    return -1;
  }
  StatusSnapshot snapshot;
  snapshot.visible = state->visible != 0;
  snapshot.pause_enabled = state->pause_enabled != 0;
  snapshot.callback = callback;
  snapshot.user_data = user_data;
  if (!utf8_to_wide(state->title, &snapshot.title) ||
      !utf8_to_wide(state->tooltip, &snapshot.tooltip) ||
      !utf8_to_wide(state->open_label, &snapshot.open_label) ||
      !utf8_to_wide(state->pause_label, &snapshot.pause_label) ||
      !utf8_to_wide(state->quit_label, &snapshot.quit_label)) {
    return -3;
  }
  const HWND window = daygo_windows_system_window();
  if (window == nullptr || !IsWindow(window)) {
    return -2;
  }
  return static_cast<int32_t>(SendMessageW(
      window, kDaygoStatusApplyMessage, 0,
      reinterpret_cast<LPARAM>(&snapshot)));
}

extern "C" void dg_status_item_stop(void) {
  const HWND window = daygo_windows_system_window();
  if (window != nullptr && IsWindow(window)) {
    SendMessageW(window, kDaygoStatusClearMessage, 0, 0);
  }
}
