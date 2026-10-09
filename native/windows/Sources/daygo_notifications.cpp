#define NOMINMAX
#define WIN32_LEAN_AND_MEAN
#define DAYGO_NOTIFICATIONS_BUILD
#include "../../include/daygo_notifications.h"
#include <windows.h>
#include <roapi.h>
#include <notificationactivationcallback.h>
#include <wrl.h>
#include <winrt/Windows.Data.Xml.Dom.h>
#include <winrt/Windows.Foundation.Collections.h>
#include <winrt/Windows.UI.Notifications.h>

#include <condition_variable>
#include <cwchar>
#include <memory>
#include <mutex>
#include <string>
#include <thread>

namespace daygo_notifications {
namespace xml = winrt::Windows::Data::Xml::Dom;
namespace toast = winrt::Windows::UI::Notifications;
using Microsoft::WRL::ClassicCom;
using Microsoft::WRL::Make;
using Microsoft::WRL::RuntimeClass;
using Microsoft::WRL::RuntimeClassFlags;
constexpr wchar_t kGroup[] = L"reminders";
constexpr int64_t kLastWindowsTick = 2650467743999999999LL;

bool is_elevated() {
  HANDLE token = nullptr;
  if (!OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, &token))
    winrt::throw_hresult(HRESULT_FROM_WIN32(GetLastError()));
  TOKEN_ELEVATION elevation{};
  DWORD bytes = 0;
  const BOOL queried = GetTokenInformation(token, TokenElevation, &elevation, sizeof(elevation), &bytes);
  const DWORD error = queried ? ERROR_SUCCESS : GetLastError();
  CloseHandle(token);
  winrt::check_hresult(HRESULT_FROM_WIN32(error));
  return elevation.TokenIsElevated != 0;
}

// Reopen/deep-link routing remains outside this delivery slice, as on macOS.
class Activation final : public RuntimeClass<RuntimeClassFlags<ClassicCom>,
                                             INotificationActivationCallback> {
 public:
  HRESULT STDMETHODCALLTYPE Activate(LPCWSTR, LPCWSTR,
      const NOTIFICATION_USER_INPUT_DATA*, ULONG) override { return S_OK; }
};
class Factory final : public RuntimeClass<RuntimeClassFlags<ClassicCom>, IClassFactory> {
 public:
  HRESULT STDMETHODCALLTYPE CreateInstance(IUnknown* outer, REFIID iid, void** out) override {
    if (!out) return E_POINTER;
    *out = nullptr;
    if (outer) return CLASS_E_NOAGGREGATION;
    auto activation = Make<Activation>();
    return activation ? activation->QueryInterface(iid, out) : E_OUTOFMEMORY;
  }
  HRESULT STDMETHODCALLTYPE LockServer(BOOL) override { return S_OK; }
};

bool valid_text(const wchar_t* value, size_t maximum, bool nonempty = false) {
  if (!value) return false;
  const size_t length = wcsnlen(value, maximum + 1);
  if (length > maximum || (nonempty && length == 0)) return false;
  for (size_t i = 0; i < length; ++i) {
    const wchar_t c = value[i];
    if (c < 0x20 && c != L'\t' && c != L'\n' && c != L'\r') return false;
    if (c == 0xfffe || c == 0xffff) return false;
    if (c >= 0xd800 && c <= 0xdbff) {
      if (++i == length || value[i] < 0xdc00 || value[i] > 0xdfff) return false;
    } else if (c >= 0xdc00 && c <= 0xdfff) return false;
  }
  return true;
}
bool valid_token(const wchar_t* token) {
  if (!valid_text(token, 16) || wcslen(token) != 16) return false;
  for (size_t i = 0; i < 16; ++i)
    if (!((token[i] >= L'0' && token[i] <= L'9') ||
          (token[i] >= L'a' && token[i] <= L'f'))) return false;
  return true;
}
bool valid_app_id(const wchar_t* value) {
  if (!valid_text(value, 128, true)) return false;
  for (const wchar_t* p = value; *p; ++p)
    if (!((*p >= L'a' && *p <= L'z') || (*p >= L'A' && *p <= L'Z') ||
          (*p >= L'0' && *p <= L'9') || *p == L'.' || *p == L'-')) return false;
  return true;
}

// DOM text/attribute setters escape user text; no caller-supplied XML is parsed.
xml::XmlDocument content(const wchar_t* id, const wchar_t* title, const wchar_t* body) {
  xml::XmlDocument doc;
  doc.LoadXml(L"<toast><visual><binding template=\"ToastGeneric\"><text/><text/></binding></visual></toast>");
  doc.DocumentElement().SetAttribute(L"launch", id);
  auto texts = doc.GetElementsByTagName(L"text");
  texts.GetAt(0).AppendChild(doc.CreateTextNode(title));
  texts.GetAt(1).AppendChild(doc.CreateTextNode(body));
  return doc;
}

struct Apartment {
  HRESULT result = RoInitialize(RO_INIT_MULTITHREADED);
  Apartment() {
    if (result != RPC_E_CHANGED_MODE) winrt::check_hresult(result);
  }
  ~Apartment() { if (SUCCEEDED(result)) RoUninitialize(); }
};

void write_registry(const std::wstring& path, const wchar_t* name, const std::wstring& value) {
  HKEY key = nullptr;
  winrt::check_hresult(HRESULT_FROM_WIN32(RegCreateKeyExW(HKEY_CURRENT_USER,
      path.c_str(), 0, nullptr, 0, KEY_SET_VALUE, nullptr, &key, nullptr)));
  const LSTATUS status = RegSetValueExW(key, name, 0, REG_SZ,
      reinterpret_cast<const BYTE*>(value.c_str()),
      static_cast<DWORD>((value.size() + 1) * sizeof(wchar_t)));
  RegCloseKey(key);
  winrt::check_hresult(HRESULT_FROM_WIN32(status));
}

struct Session {
  std::wstring app_id;
  std::wstring clsid_string;
  std::wstring executable;
  GUID clsid{};
  std::mutex state_mu;
  std::condition_variable changed;
  HRESULT started = E_PENDING;
  bool ready = false;
  bool stop = false;
  std::thread worker;
  std::mutex operations;

  void run() noexcept {
    DWORD cookie = 0;
    HRESULT result = S_OK;
    const HRESULT apartment = RoInitialize(RO_INIT_MULTITHREADED);
    try {
      winrt::check_hresult(apartment);
      auto factory = Make<Factory>();
      if (!factory) winrt::throw_hresult(E_OUTOFMEMORY);
      winrt::check_hresult(CoRegisterClassObject(clsid, factory.Get(), CLSCTX_LOCAL_SERVER,
          REGCLS_MULTIPLEUSE, &cookie));
      write_registry(L"Software\\Classes\\CLSID\\" + clsid_string + L"\\LocalServer32",
          nullptr, L"\"" + executable + L"\" --daygo-toast-activated");
      const std::wstring key = L"Software\\Classes\\AppUserModelId\\" + app_id;
      write_registry(key, L"DisplayName", L"Daygo");
      write_registry(key, L"CustomActivator", clsid_string);
      // Probe implementation only. A disabled notification setting is still a
      // supported capability; permission is queried separately at delivery.
      toast::ToastNotificationManager::CreateToastNotifier(app_id);
    } catch (const winrt::hresult_error& failure) { result = failure.code(); }
      catch (...) { result = E_FAIL; }
    {
      std::unique_lock<std::mutex> lock(state_mu);
      started = result;
      ready = true;
      changed.notify_all();
      if (SUCCEEDED(result)) changed.wait(lock, [this] { return stop; });
    }
    if (cookie) CoRevokeClassObject(cookie);
    if (SUCCEEDED(apartment)) RoUninitialize();
  }
  ~Session() {
    {
      std::lock_guard<std::mutex> lock(state_mu);
      stop = true;
    }
    changed.notify_all();
    if (worker.joinable()) worker.join();
  }
};

bool matches(const xml::XmlDocument& document, const wchar_t* id) {
  return document.DocumentElement().GetAttribute(L"launch") == id;
}
// First scan *all* matching tokens. A truncated-hash collision must never
// remove another ID's schedule or notification history.
void check_collision(Session& s, const toast::ToastNotifier& notifier,
                     const wchar_t* id, const wchar_t* token) {
  for (auto pending : notifier.GetScheduledToastNotifications())
    if (pending.Group() == kGroup && (pending.Id() == token || pending.Tag() == token) &&
        !matches(pending.Content(), id)) winrt::throw_hresult(E_INVALIDARG);
  for (auto shown : toast::ToastNotificationManager::History().GetHistory(s.app_id))
    if (shown.Group() == kGroup && shown.Tag() == token &&
        !matches(shown.Content(), id)) winrt::throw_hresult(E_INVALIDARG);
}
void remove(Session& s, const toast::ToastNotifier& notifier,
            const wchar_t* id, const wchar_t* token) {
  check_collision(s, notifier, id, token);
  for (auto pending : notifier.GetScheduledToastNotifications())
    if (pending.Group() == kGroup && pending.Id() == token && matches(pending.Content(), id))
      notifier.RemoveFromSchedule(pending);
  toast::ToastNotificationManager::History().Remove(token, kGroup, s.app_id);
}

template <typename Fn>
int32_t call(uint32_t abi, void* handle, int32_t* error, Fn fn) noexcept {
  if (error) *error = 0;
  if (abi != DG_NOTIFICATION_ABI_MAJOR) return DG_NOTIFICATION_UNAVAILABLE;
  if (!handle || !error) return DG_NOTIFICATION_INVALID;
  try {
    Apartment apartment;
    auto& session = *static_cast<Session*>(handle);
    std::lock_guard<std::mutex> lock(session.operations);
    return fn(session);
  } catch (const winrt::hresult_error& failure) {
    *error = failure.code();
    return failure.code() == E_INVALIDARG ? DG_NOTIFICATION_INVALID : DG_NOTIFICATION_FAILED;
  } catch (...) { *error = E_FAIL; return DG_NOTIFICATION_FAILED; }
}
} // namespace daygo_notifications

extern "C" {
int32_t dg_notification_open(uint32_t abi, const wchar_t* app_id, const wchar_t* clsid,
    const wchar_t* executable, void** handle, int32_t* error) {
  using namespace daygo_notifications;
  if (handle) *handle = nullptr;
  if (error) *error = 0;
  if (abi != DG_NOTIFICATION_ABI_MAJOR) return DG_NOTIFICATION_UNAVAILABLE;
  GUID guid{};
  if (!handle || !error || !valid_app_id(app_id) || !valid_text(clsid, 38, true) ||
      FAILED(CLSIDFromString(clsid, &guid)) || !valid_text(executable, 32768, true) ||
      wcschr(executable, L'\"') || wcslen(executable) < 3 || executable[1] != L':' ||
      (executable[2] != L'\\' && executable[2] != L'/')) return DG_NOTIFICATION_INVALID;
  try {
    // Windows desktop app notifications do not support an elevated sender.
    // Keep this environment out of NotificationAvailability rather than
    // advertising delivery and failing later at ToastNotifier.Setting/Show.
    if (is_elevated()) return DG_NOTIFICATION_UNAVAILABLE;
    auto session = std::make_unique<Session>();
    session->app_id = app_id;
    session->clsid_string = clsid;
    session->clsid = guid;
    session->executable = executable;
    session->worker = std::thread([s = session.get()] { s->run(); });
    std::unique_lock<std::mutex> lock(session->state_mu);
    session->changed.wait(lock, [&session] { return session->ready; });
    const HRESULT result = session->started;
    lock.unlock();
    if (FAILED(result)) { *error = result; return DG_NOTIFICATION_FAILED; }
    *handle = session.release();
    return DG_NOTIFICATION_OK;
  } catch (const winrt::hresult_error& failure) { *error = failure.code(); }
    catch (...) { *error = E_FAIL; }
  return DG_NOTIFICATION_FAILED;
}
void dg_notification_close(void* session) {
  delete static_cast<daygo_notifications::Session*>(session);
}
int32_t dg_notification_permission(uint32_t abi, void* handle, uint32_t* permission, int32_t* error) {
  using namespace daygo_notifications;
  if (permission) *permission = 0;
  return call(abi, handle, error, [permission](Session& s) -> int32_t {
    if (!permission) return DG_NOTIFICATION_INVALID;
    auto notifier = toast::ToastNotificationManager::CreateToastNotifier(s.app_id);
    *permission = notifier.Setting() == toast::NotificationSetting::Enabled ? 1 : 2;
    return DG_NOTIFICATION_OK;
  });
}
int32_t dg_notification_schedule(uint32_t abi, void* handle, const wchar_t* id,
    const wchar_t* token, const wchar_t* title, const wchar_t* body,
    int64_t ticks, int32_t* error) {
  using namespace daygo_notifications;
  return call(abi, handle, error, [&](Session& s) -> int32_t {
    if (!valid_text(id, 4096, true) || !valid_token(token) || !valid_text(title, 4096) ||
        !valid_text(body, 16384) || ticks < 0 || ticks > kLastWindowsTick) return DG_NOTIFICATION_INVALID;
    auto document = content(id, title, body);
    auto notifier = toast::ToastNotificationManager::CreateToastNotifier(s.app_id);
    if (notifier.Setting() != toast::NotificationSetting::Enabled) return DG_NOTIFICATION_DENIED;
    // Build/validate before removing the previous schedule. If AddToSchedule
    // fails, Go remains unarmed and retries; no duplicate can survive removal.
    if (ticks > winrt::clock::now().time_since_epoch().count()) {
      toast::ScheduledToastNotification notification(document,
          winrt::clock::time_point{winrt::clock::duration{ticks}});
      notification.Id(token);
      notification.Tag(token);
      notification.Group(kGroup);
      remove(s, notifier, id, token);
      notifier.AddToSchedule(notification);
    } else {
      toast::ToastNotification notification(document);
      notification.Tag(token);
      notification.Group(kGroup);
      remove(s, notifier, id, token);
      notifier.Show(notification);
    }
    return DG_NOTIFICATION_OK;
  });
}
int32_t dg_notification_cancel(uint32_t abi, void* handle, const wchar_t* id,
    const wchar_t* token, int32_t* error) {
  using namespace daygo_notifications;
  return call(abi, handle, error, [&](Session& s) -> int32_t {
    if (!valid_text(id, 4096, true) || !valid_token(token)) return DG_NOTIFICATION_INVALID;
    auto notifier = toast::ToastNotificationManager::CreateToastNotifier(s.app_id);
    remove(s, notifier, id, token);
    return DG_NOTIFICATION_OK;
  });
}
} // extern "C"
