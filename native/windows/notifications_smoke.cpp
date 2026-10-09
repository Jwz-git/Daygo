// Anonymous ABI/XML/OS-scheduling fixture. Never uses Daygo's product identity
// and never schedules anything near the current time or displays a toast.
#include "Sources/daygo_notifications.cpp"
#include <cstdio>
#include <stdexcept>

void require(bool condition, const char* message) {
  if (!condition) throw std::runtime_error(message);
}
struct Fixture {
  std::wstring app_id;
  std::wstring clsid;
  void* session = nullptr;
  Fixture() {
    GUID guid{};
    winrt::check_hresult(CoCreateGuid(&guid));
    wchar_t text[40]{};
    require(StringFromGUID2(guid, text, 40) != 0, "guid conversion failed");
    clsid = text;
    app_id = L"DaygoNotificationFixture.";
    for (const wchar_t c : clsid) if (c != L'{' && c != L'}') app_id += c;
  }
  ~Fixture() {
    // Cancel only the anonymous fixture's schedules even if an assertion fails.
    try {
      auto notifier = daygo_notifications::toast::ToastNotificationManager::CreateToastNotifier(app_id);
      for (auto n : notifier.GetScheduledToastNotifications()) notifier.RemoveFromSchedule(n);
      daygo_notifications::toast::ToastNotificationManager::History().Clear(app_id);
    } catch (...) {}
    dg_notification_close(session);
    RegDeleteTreeW(HKEY_CURRENT_USER, (L"Software\\Classes\\AppUserModelId\\" + app_id).c_str());
    RegDeleteTreeW(HKEY_CURRENT_USER, (L"Software\\Classes\\CLSID\\" + clsid).c_str());
  }
};
int main() {
  using namespace daygo_notifications;
  try {
    Apartment apartment;
    auto doc = content(L"fixture<&\"", L"匿名 <text> & 引号\"", L"</text><actions/>匿名正文");
    require(doc.GetElementsByTagName(L"text").Length() == 2, "text became XML nodes");
    require(doc.GetElementsByTagName(L"actions").Length() == 0, "body became actions");
    require(doc.GetElementsByTagName(L"text").GetAt(0).InnerText() == L"匿名 <text> & 引号\"", "text roundtrip failed");
    require(matches(doc, L"fixture<&\""), "full identifier roundtrip failed");
    require(!valid_text(L"bad\x01", 4096), "control character accepted");
    require(!valid_text(L"\xd800", 4096), "unpaired surrogate accepted");
    require(valid_text(L"\xd83d\xde00", 4096), "valid surrogate rejected");
    require(valid_token(L"0123456789abcdef") && !valid_token(L"0123456789abcdeg"), "token validation failed");
    int32_t error = 99;
    void* handle = reinterpret_cast<void*>(1);
    require(dg_notification_open(999, nullptr, nullptr, nullptr, &handle, &error) == DG_NOTIFICATION_UNAVAILABLE && !handle && !error, "ABI mismatch accepted");
    require(dg_notification_open(1, L"bad/id", L"bad", L"bad", &handle, &error) == DG_NOTIFICATION_INVALID && !handle, "invalid registration accepted");
    uint32_t permission = 99;
    require(dg_notification_permission(1, nullptr, &permission, &error) == DG_NOTIFICATION_INVALID && permission == 0, "invalid handle accepted");

    Fixture fixture;
    wchar_t executable[32768]{};
    const DWORD length = GetModuleFileNameW(nullptr, executable, 32768);
    require(length != 0 && length < 32768, "fixture executable unavailable");
    const int32_t opened = dg_notification_open(1, fixture.app_id.c_str(), fixture.clsid.c_str(), executable, &fixture.session, &error);
    if (opened != DG_NOTIFICATION_OK) {
      std::fprintf(stderr, "native registration status=%d HRESULT=0x%08x\n", opened, static_cast<uint32_t>(error));
      return 1;
    }
    require(dg_notification_permission(1, fixture.session, &permission, &error) == DG_NOTIFICATION_OK, "permission query failed");
    require(permission == 1 || permission == 2, "invalid permission state");
    require(dg_notification_schedule(1, fixture.session, L"fixture", L"bad", L"anonymous", L"anonymous", 0, &error) == DG_NOTIFICATION_INVALID, "invalid token accepted");
    if (permission == 2) {
      require(dg_notification_schedule(1, fixture.session, L"fixture", L"0123456789abcdef", L"anonymous", L"anonymous", 0, &error) == DG_NOTIFICATION_DENIED, "disabled setting accepted");
      std::puts("SKIP OS schedule replacement/cancel: runner notification setting is disabled; denied delivery verified");
    } else {
      const int64_t at = winrt::clock::now().time_since_epoch().count() + 1800LL * 10000000;
      const wchar_t* token = L"0123456789abcdef";
      require(dg_notification_schedule(1, fixture.session, L"fixture", token, L"anonymous", L"anonymous", at, &error) == DG_NOTIFICATION_OK, "schedule failed");
      require(dg_notification_schedule(1, fixture.session, L"fixture", token, L"anonymous changed", L"anonymous", at + 10000000, &error) == DG_NOTIFICATION_OK, "reschedule failed");
      auto notifier = toast::ToastNotificationManager::CreateToastNotifier(fixture.app_id);
      auto pending = notifier.GetScheduledToastNotifications();
      require(pending.Size() == 1 && pending.GetAt(0).DeliveryTime().time_since_epoch().count() == at + 10000000, "reschedule did not replace exactly once");
      require(dg_notification_schedule(1, fixture.session, L"different-id", token, L"anonymous", L"anonymous", at, &error) == DG_NOTIFICATION_INVALID, "collision accepted");
      require(dg_notification_cancel(1, fixture.session, L"different-id", token, &error) == DG_NOTIFICATION_INVALID, "collision cancelled another ID");
      require(notifier.GetScheduledToastNotifications().Size() == 1, "collision changed schedule");
      require(dg_notification_cancel(1, fixture.session, L"fixture", token, &error) == DG_NOTIFICATION_OK, "cancel failed");
      require(dg_notification_cancel(1, fixture.session, L"fixture", token, &error) == DG_NOTIFICATION_OK, "cancel was not idempotent");
      require(notifier.GetScheduledToastNotifications().Size() == 0, "cancel retained schedule");
      std::puts("OS future schedule replacement/cancel passed (no notification displayed)");
    }
    dg_notification_close(fixture.session);
    fixture.session = nullptr;
    std::puts("native notification ABI/XML/registration/teardown contracts passed; real desktop delivery not exercised");
    return 0;
  } catch (const winrt::hresult_error& failure) {
    std::fprintf(stderr, "native fixture HRESULT=0x%08x\n", static_cast<uint32_t>(failure.code()));
  } catch (const std::exception& failure) { std::fprintf(stderr, "%s\n", failure.what()); }
  return 1;
}
