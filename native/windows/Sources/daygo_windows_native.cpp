#define NOMINMAX
#define WIN32_LEAN_AND_MEAN
#define DAYGO_APPLICATION_BUILD 1
#define DAYGO_CAPTURE_STATIC 1
#include "../../include/daygo_application.h"
#include "../../include/daygo_capture.h"

#include <windows.h>
#include <bcrypt.h>
#include <d3d11.h>
#include <dxgi1_2.h>
#include <roapi.h>
#include <shellapi.h>
#include <shlobj.h>
#include <tlhelp32.h>
#include <winternl.h>
#include <wincodec.h>
#include <windows.graphics.capture.interop.h>
#include <windows.graphics.directx.direct3d11.interop.h>
#include <windows.ui.interop.h>

#include <winrt/Windows.Foundation.Collections.h>
#include <winrt/Windows.Graphics.Capture.h>
#include <winrt/Windows.Graphics.DirectX.Direct3D11.h>
#include <winrt/Windows.Graphics.DirectX.h>
#include <winrt/Windows.UI.h>
#include <winrt/base.h>

#include <algorithm>
#include <chrono>
#include <condition_variable>
#include <cstdint>
#include <cwctype>
#include <iterator>
#include <mutex>
#include <string>
#include <unordered_map>
#include <unordered_set>
#include <vector>

using namespace std::chrono_literals;
namespace wfc = winrt::Windows::Foundation::Collections;
namespace wgc = winrt::Windows::Graphics::Capture;
namespace wgd = winrt::Windows::Graphics::DirectX;
namespace wgd3d = winrt::Windows::Graphics::DirectX::Direct3D11;
namespace wui = winrt::Windows::UI;

namespace {

constexpr uint32_t kMinimumPrivacyBuild = 26100;
constexpr size_t kIdentityPrefixLength = 17;
constexpr size_t kIdentityLength = kIdentityPrefixLength + 64;
constexpr size_t kMaximumPathBytes = 32768;
constexpr size_t kMaximumIdentifierBytes = 4096;
constexpr size_t kMaximumNameBytes = 4096;
constexpr size_t kMaximumIconBytes = 256 * 1024;

template <typename T>
class Handle {
 public:
  Handle() = default;
  explicit Handle(T value) : value_(value) {}
  ~Handle() { reset(); }
  Handle(const Handle&) = delete;
  Handle& operator=(const Handle&) = delete;
  Handle(Handle&& other) noexcept : value_(other.release()) {}
  Handle& operator=(Handle&& other) noexcept {
    if (this != &other) reset(other.release());
    return *this;
  }
  T get() const { return value_; }
  explicit operator bool() const { return value_ && value_ != INVALID_HANDLE_VALUE; }
  T release() {
    T value = value_;
    value_ = {};
    return value;
  }
  void reset(T value = {}) {
    if (*this) CloseHandle(value_);
    value_ = value;
  }

 private:
  T value_{};
};

struct Apartment {
  HRESULT result = CoInitializeEx(nullptr, COINIT_MULTITHREADED);
  ~Apartment() {
    if (result == S_OK || result == S_FALSE) CoUninitialize();
  }
  bool usable() const {
    return SUCCEEDED(result) || result == RPC_E_CHANGED_MODE;
  }
};

struct ApplicationData {
  std::string identifier;
  std::string name;
  std::vector<uint8_t> icon_png;
};

std::mutex application_cache_mutex;
std::unordered_map<std::string, ApplicationData> application_cache;

bool utf8_to_wide(const uint8_t* data, uint64_t length, size_t maximum,
                  std::wstring* output) {
  if (!data || length == 0 || length > maximum || length > INT_MAX) return false;
  const int count = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS,
                                         reinterpret_cast<const char*>(data),
                                         static_cast<int>(length), nullptr, 0);
  if (count <= 0) return false;
  output->resize(static_cast<size_t>(count));
  return MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS,
                             reinterpret_cast<const char*>(data),
                             static_cast<int>(length), output->data(), count) == count &&
         output->find(L'\0') == std::wstring::npos;
}

std::string wide_to_utf8(const std::wstring& value) {
  if (value.empty()) return {};
  const int count = WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS,
                                        value.data(), static_cast<int>(value.size()),
                                        nullptr, 0, nullptr, nullptr);
  if (count <= 0) return {};
  std::string result(static_cast<size_t>(count), '\0');
  if (WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, value.data(),
                          static_cast<int>(value.size()), result.data(), count,
                          nullptr, nullptr) != count) {
    return {};
  }
  return result;
}

bool canonical_executable_path(const std::wstring& input, std::wstring* output) {
  if (input.empty()) return false;
  const DWORD required = GetFullPathNameW(input.c_str(), 0, nullptr, nullptr);
  if (required == 0 || required > 32768) return false;
  std::wstring absolute(required, L'\0');
  const DWORD written = GetFullPathNameW(input.c_str(), required, absolute.data(), nullptr);
  if (written == 0 || written >= required) return false;
  absolute.resize(written);

  Handle<HANDLE> file(CreateFileW(absolute.c_str(), FILE_READ_ATTRIBUTES,
                                   FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE,
                                   nullptr, OPEN_EXISTING,
                                   FILE_ATTRIBUTE_NORMAL, nullptr));
  if (!file) return false;
  BY_HANDLE_FILE_INFORMATION information{};
  if (!GetFileInformationByHandle(file.get(), &information) ||
      (information.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY)) {
    return false;
  }
  const DWORD final_required = GetFinalPathNameByHandleW(
      file.get(), nullptr, 0, FILE_NAME_NORMALIZED | VOLUME_NAME_DOS);
  if (final_required > 0 && final_required <= 32768) {
    std::wstring final_path(final_required, L'\0');
    const DWORD final_written = GetFinalPathNameByHandleW(
        file.get(), final_path.data(), final_required,
        FILE_NAME_NORMALIZED | VOLUME_NAME_DOS);
    if (final_written > 0 && final_written < final_required) {
      final_path.resize(final_written);
      if (final_path.rfind(L"\\\\?\\", 0) == 0) final_path.erase(0, 4);
      absolute = std::move(final_path);
    }
  }
  std::replace(absolute.begin(), absolute.end(), L'/', L'\\');
  std::transform(absolute.begin(), absolute.end(), absolute.begin(), towlower);
  *output = std::move(absolute);
  return true;
}

bool has_exe_extension(const std::wstring& path) {
  const size_t dot = path.find_last_of(L'.');
  if (dot == std::wstring::npos) return false;
  std::wstring extension = path.substr(dot);
  std::transform(extension.begin(), extension.end(), extension.begin(), towlower);
  return extension == L".exe";
}

bool sha256(const uint8_t* data, size_t length, std::vector<uint8_t>* digest) {
  BCRYPT_ALG_HANDLE algorithm = nullptr;
  BCRYPT_HASH_HANDLE hash = nullptr;
  DWORD object_size = 0;
  DWORD bytes = 0;
  if (BCryptOpenAlgorithmProvider(&algorithm, BCRYPT_SHA256_ALGORITHM, nullptr, 0) < 0) return false;
  const NTSTATUS property = BCryptGetProperty(
      algorithm, BCRYPT_OBJECT_LENGTH, reinterpret_cast<PUCHAR>(&object_size),
      sizeof(object_size), &bytes, 0);
  std::vector<uint8_t> object(object_size);
  digest->assign(32, 0);
  NTSTATUS status = property;
  if (status >= 0) status = BCryptCreateHash(algorithm, &hash, object.data(), object_size, nullptr, 0, 0);
  if (status >= 0) status = BCryptHashData(hash, const_cast<PUCHAR>(data), static_cast<ULONG>(length), 0);
  if (status >= 0) status = BCryptFinishHash(hash, digest->data(), static_cast<ULONG>(digest->size()), 0);
  if (hash) BCryptDestroyHash(hash);
  BCryptCloseAlgorithmProvider(algorithm, 0);
  return status >= 0;
}

std::string application_identifier(const std::wstring& canonical_path) {
  const std::string path = wide_to_utf8(canonical_path);
  std::vector<uint8_t> digest;
  if (path.empty() || !sha256(reinterpret_cast<const uint8_t*>(path.data()), path.size(), &digest)) return {};
  static constexpr char digits[] = "0123456789abcdef";
  std::string result = "win32.exe.sha256:";
  result.reserve(result.size() + digest.size() * 2);
  for (uint8_t value : digest) {
    result.push_back(digits[value >> 4]);
    result.push_back(digits[value & 0x0f]);
  }
  return result;
}

std::wstring file_display_name(const std::wstring& path) {
  DWORD ignored = 0;
  const DWORD size = GetFileVersionInfoSizeW(path.c_str(), &ignored);
  if (size > 0) {
    std::vector<uint8_t> version(size);
    if (GetFileVersionInfoW(path.c_str(), 0, size, version.data())) {
      struct Translation { WORD language; WORD code_page; };
      Translation* translations = nullptr;
      UINT translation_bytes = 0;
      if (VerQueryValueW(version.data(), L"\\VarFileInfo\\Translation",
                         reinterpret_cast<void**>(&translations), &translation_bytes) &&
          translation_bytes >= sizeof(Translation)) {
        const wchar_t* keys[] = {L"FileDescription", L"ProductName"};
        for (const wchar_t* key : keys) {
          wchar_t query[96] = {};
          swprintf_s(query, L"\\StringFileInfo\\%04x%04x\\%s",
                     translations[0].language, translations[0].code_page, key);
          wchar_t* value = nullptr;
          UINT value_chars = 0;
          if (VerQueryValueW(version.data(), query, reinterpret_cast<void**>(&value),
                             &value_chars) && value && value_chars > 0) {
            // VerQueryValue reports puLen in bytes for some resources and in
            // characters for others — Spotify's version block answers 16 for the
            // seven-character "Spotify" — so it cannot be trusted as a character
            // count: subtracting one from a byte count runs the name past its
            // terminator and into the neighbouring string values. The value is
            // NUL-terminated, so the name ends at the terminator and puLen only
            // bounds the read.
            const size_t limit = value_chars - 1;
            size_t length = 0;
            while (length < limit && value[length] != L'\0') ++length;
            if (length > 0) return std::wstring(value, length);
          }
        }
      }
    }
  }
  const size_t slash = path.find_last_of(L"\\/");
  const size_t dot = path.find_last_of(L'.');
  const size_t start = slash == std::wstring::npos ? 0 : slash + 1;
  const size_t end = dot == std::wstring::npos || dot < start ? path.size() : dot;
  return path.substr(start, end - start);
}

std::vector<uint8_t> application_icon_png(const std::wstring& path) {
  SHFILEINFOW info{};
  if (!SHGetFileInfoW(path.c_str(), FILE_ATTRIBUTE_NORMAL, &info, sizeof(info),
                      SHGFI_ICON | SHGFI_LARGEICON)) {
    return {};
  }
  HICON icon = info.hIcon;
  if (!icon) return {};
  constexpr UINT side = 64;
  BITMAPINFO bitmap_info{};
  bitmap_info.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
  bitmap_info.bmiHeader.biWidth = side;
  bitmap_info.bmiHeader.biHeight = -static_cast<LONG>(side);
  bitmap_info.bmiHeader.biPlanes = 1;
  bitmap_info.bmiHeader.biBitCount = 32;
  bitmap_info.bmiHeader.biCompression = BI_RGB;
  void* bits = nullptr;
  HDC dc = CreateCompatibleDC(nullptr);
  HBITMAP bitmap = dc ? CreateDIBSection(dc, &bitmap_info, DIB_RGB_COLORS, &bits, nullptr, 0) : nullptr;
  if (!dc || !bitmap || !bits) {
    if (bitmap) DeleteObject(bitmap);
    if (dc) DeleteDC(dc);
    DestroyIcon(icon);
    return {};
  }
  memset(bits, 0, side * side * 4);
  HGDIOBJ old = SelectObject(dc, bitmap);
  const BOOL drawn = DrawIconEx(dc, 0, 0, icon, side, side, 0, nullptr, DI_NORMAL);

  std::vector<uint8_t> png;
  if (drawn) {
    winrt::com_ptr<IWICImagingFactory> factory;
    winrt::com_ptr<IStream> stream;
    winrt::com_ptr<IWICBitmapEncoder> encoder;
    winrt::com_ptr<IWICBitmapFrameEncode> frame;
    winrt::com_ptr<IWICBitmap> source;
    HGLOBAL memory = GlobalAlloc(GMEM_MOVEABLE, 0);
    HRESULT hr = memory ? CreateStreamOnHGlobal(memory, TRUE, stream.put()) : E_OUTOFMEMORY;
    if (SUCCEEDED(hr)) hr = CoCreateInstance(CLSID_WICImagingFactory, nullptr, CLSCTX_INPROC_SERVER, IID_PPV_ARGS(factory.put()));
    if (SUCCEEDED(hr)) hr = factory->CreateEncoder(GUID_ContainerFormatPng, nullptr, encoder.put());
    if (SUCCEEDED(hr)) hr = encoder->Initialize(stream.get(), WICBitmapEncoderNoCache);
    if (SUCCEEDED(hr)) hr = encoder->CreateNewFrame(frame.put(), nullptr);
    if (SUCCEEDED(hr)) hr = frame->Initialize(nullptr);
    if (SUCCEEDED(hr)) hr = frame->SetSize(side, side);
    WICPixelFormatGUID format = GUID_WICPixelFormat32bppBGRA;
    if (SUCCEEDED(hr)) hr = frame->SetPixelFormat(&format);
    if (SUCCEEDED(hr)) hr = factory->CreateBitmapFromMemory(
        side, side, GUID_WICPixelFormat32bppBGRA, side * 4, side * side * 4,
        static_cast<BYTE*>(bits), source.put());
    if (SUCCEEDED(hr)) hr = frame->WriteSource(source.get(), nullptr);
    if (SUCCEEDED(hr)) hr = frame->Commit();
    if (SUCCEEDED(hr)) hr = encoder->Commit();
    if (SUCCEEDED(hr)) {
      STATSTG stat{};
      if (SUCCEEDED(stream->Stat(&stat, STATFLAG_NONAME)) &&
          stat.cbSize.QuadPart > 0 && stat.cbSize.QuadPart <= kMaximumIconBytes) {
        LARGE_INTEGER zero{};
        stream->Seek(zero, STREAM_SEEK_SET, nullptr);
        png.resize(static_cast<size_t>(stat.cbSize.QuadPart));
        ULONG read = 0;
        if (FAILED(stream->Read(png.data(), static_cast<ULONG>(png.size()), &read)) ||
            read != png.size()) {
          png.clear();
        }
      }
    }
  }
  SelectObject(dc, old);
  DeleteObject(bitmap);
  DeleteDC(dc);
  DestroyIcon(icon);
  return png;
}

bool inspect_path(const std::wstring& supplied, ApplicationData* application) {
  std::wstring path;
  if (!canonical_executable_path(supplied, &path) || !has_exe_extension(path)) return false;
  application->identifier = application_identifier(path);
  application->name = wide_to_utf8(file_display_name(path));
  if (application->identifier.empty() || application->name.empty()) return false;
  Apartment apartment;
  if (apartment.usable()) application->icon_png = application_icon_png(path);
  return true;
}

std::wstring process_path(DWORD process_id) {
  Handle<HANDLE> process(OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, process_id));
  if (!process) return {};
  std::wstring value(32768, L'\0');
  DWORD size = static_cast<DWORD>(value.size());
  if (!QueryFullProcessImageNameW(process.get(), 0, value.data(), &size)) return {};
  value.resize(size);
  std::wstring canonical;
  return canonical_executable_path(value, &canonical) ? canonical : std::wstring{};
}

bool inspect_if_identifier_matches(const std::wstring& candidate,
                                   const std::string& identifier,
                                   ApplicationData* application) {
  std::wstring path = candidate;
  if (path.size() >= 2 && path.front() == L'"') {
    const size_t closing = path.find(L'"', 1);
    if (closing != std::wstring::npos) path = path.substr(1, closing - 1);
  } else {
    const size_t comma = path.rfind(L',');
    if (comma != std::wstring::npos) path.resize(comma);
  }
  while (!path.empty() && iswspace(path.back())) path.pop_back();
  std::wstring canonical;
  if (!canonical_executable_path(path, &canonical) ||
      application_identifier(canonical) != identifier) {
    return false;
  }
  return inspect_path(canonical, application);
}

bool search_registry_values(HKEY root, const wchar_t* key_path, REGSAM view,
                            const wchar_t* value_name,
                            const std::string& identifier,
                            ApplicationData* application) {
  HKEY raw_key = nullptr;
  if (RegOpenKeyExW(root, key_path, 0, KEY_READ | view, &raw_key) != ERROR_SUCCESS) return false;
  struct RegistryKey {
    HKEY value;
    ~RegistryKey() { if (value) RegCloseKey(value); }
  } key{raw_key};
  for (DWORD index = 0;; ++index) {
    wchar_t subkey_name[512] = {};
    DWORD subkey_chars = static_cast<DWORD>(std::size(subkey_name));
    const LONG enumerated = RegEnumKeyExW(key.value, index, subkey_name,
                                         &subkey_chars, nullptr, nullptr, nullptr, nullptr);
    if (enumerated == ERROR_NO_MORE_ITEMS) break;
    if (enumerated != ERROR_SUCCESS) continue;
    HKEY raw_subkey = nullptr;
    if (RegOpenKeyExW(key.value, subkey_name, 0, KEY_READ | view, &raw_subkey) != ERROR_SUCCESS) continue;
    RegistryKey subkey{raw_subkey};
    DWORD type = 0;
    DWORD byte_count = 0;
    if (RegQueryValueExW(subkey.value, value_name, nullptr, &type, nullptr,
                         &byte_count) != ERROR_SUCCESS ||
        (type != REG_SZ && type != REG_EXPAND_SZ) || byte_count < sizeof(wchar_t) ||
        byte_count > 32768 * sizeof(wchar_t)) {
      continue;
    }
    std::wstring value(byte_count / sizeof(wchar_t), L'\0');
    if (RegQueryValueExW(subkey.value, value_name, nullptr, &type,
                         reinterpret_cast<BYTE*>(value.data()), &byte_count) != ERROR_SUCCESS) {
      continue;
    }
    value.resize(wcsnlen_s(value.c_str(), value.size()));
    if (type == REG_EXPAND_SZ) {
      const DWORD required = ExpandEnvironmentStringsW(value.c_str(), nullptr, 0);
      if (required > 0 && required <= 32768) {
        std::wstring expanded(required, L'\0');
        const DWORD written = ExpandEnvironmentStringsW(value.c_str(), expanded.data(), required);
        if (written > 0 && written <= required) {
          expanded.resize(written - 1);
          value = std::move(expanded);
        }
      }
    }
    if (inspect_if_identifier_matches(value, identifier, application)) return true;
  }
  return false;
}

bool find_registered_application(const std::string& identifier,
                                 ApplicationData* application) {
  constexpr wchar_t app_paths[] =
      L"SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\App Paths";
  constexpr wchar_t uninstall[] =
      L"SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall";
  const HKEY roots[] = {HKEY_CURRENT_USER, HKEY_LOCAL_MACHINE};
  const REGSAM views[] = {KEY_WOW64_64KEY, KEY_WOW64_32KEY};
  for (HKEY root : roots) {
    for (REGSAM view : views) {
      if (search_registry_values(root, app_paths, view, nullptr, identifier, application) ||
          search_registry_values(root, uninstall, view, L"DisplayIcon", identifier, application)) {
        return true;
      }
    }
  }
  return false;
}

bool find_application_by_identifier(const std::string& identifier,
                                    ApplicationData* application) {
  {
    std::lock_guard lock(application_cache_mutex);
    const auto found = application_cache.find(identifier);
    if (found != application_cache.end()) {
      *application = found->second;
      return true;
    }
  }
  Handle<HANDLE> snapshot(CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0));
  if (snapshot) {
    PROCESSENTRY32W process{};
    process.dwSize = sizeof(process);
    if (Process32FirstW(snapshot.get(), &process)) {
      do {
        const std::wstring path = process_path(process.th32ProcessID);
        if (!path.empty() && application_identifier(path) == identifier &&
            inspect_path(path, application)) {
          std::lock_guard lock(application_cache_mutex);
          application_cache[identifier] = *application;
          return true;
        }
      } while (Process32NextW(snapshot.get(), &process));
    }
  }
  if (find_registered_application(identifier, application)) {
    std::lock_guard lock(application_cache_mutex);
    application_cache[identifier] = *application;
    return true;
  }
  return false;
}

void clear_application_info(dg_application_info_v2* info) {
  if (!info) return;
  info->identifier.len = 0;
  info->name.len = 0;
  info->icon_png.len = 0;
}

void clear_application_error(dg_application_error_v1* error) {
  if (!error || error->struct_size < sizeof(*error)) return;
  error->native_domain = DG_APPLICATION_NATIVE_NONE;
  error->native_code = 0;
}

int32_t application_failure(int32_t status, HRESULT native,
                            dg_application_error_v1* error) {
  if (error && error->struct_size >= sizeof(*error)) {
    error->native_domain = native == S_OK ? DG_APPLICATION_NATIVE_NONE
                                          : DG_APPLICATION_NATIVE_WINDOWS;
    error->native_code = native;
  }
  return status;
}

bool write_buffer(const std::string& value, dg_application_buffer_v1* buffer) {
  if (value.empty() || !buffer || !buffer->data || value.size() > buffer->capacity) return false;
  memcpy(buffer->data, value.data(), value.size());
  buffer->len = value.size();
  return true;
}

int32_t write_application(const ApplicationData& application,
                          dg_application_info_v2* info,
                          dg_application_error_v1* error) {
  if (!write_buffer(application.identifier, &info->identifier) ||
      !write_buffer(application.name, &info->name)) {
    clear_application_info(info);
    return application_failure(DG_APPLICATION_E_INVALID_ARGUMENT, S_OK, error);
  }
  if (!application.icon_png.empty() && info->icon_png.data &&
      application.icon_png.size() <= info->icon_png.capacity) {
    memcpy(info->icon_png.data, application.icon_png.data(), application.icon_png.size());
    info->icon_png.len = application.icon_png.size();
  }
  return DG_APPLICATION_OK;
}

bool valid_application_call(uint32_t requested_abi_major,
                            dg_application_info_v2* info,
                            dg_application_error_v1* error) {
  if (!info || info->struct_size != sizeof(*info) || info->reserved0 != 0 ||
      (error && error->struct_size < sizeof(*error))) return false;
  clear_application_info(info);
  clear_application_error(error);
  return requested_abi_major == DG_APPLICATION_ABI_MAJOR;
}

uint32_t windows_build_number() {
  using RtlGetVersionFn = LONG(WINAPI*)(PRTL_OSVERSIONINFOW);
  HMODULE ntdll = GetModuleHandleW(L"ntdll.dll");
  auto function = ntdll ? reinterpret_cast<RtlGetVersionFn>(
                              GetProcAddress(ntdll, "RtlGetVersion"))
                        : nullptr;
  RTL_OSVERSIONINFOW version{};
  version.dwOSVersionInfoSize = sizeof(version);
  return function && function(&version) == 0 ? version.dwBuildNumber : 0;
}

bool valid_windows_identifier(const std::string& value) {
  if (value.size() != kIdentityLength || value.rfind("win32.exe.sha256:", 0) != 0) return false;
  return std::all_of(value.begin() + kIdentityPrefixLength, value.end(), [](char character) {
    return (character >= '0' && character <= '9') ||
           (character >= 'a' && character <= 'f');
  });
}

std::unordered_set<std::string> decode_blocked_identifiers(
    const dg_capture_request_v1& request, bool* valid) {
  std::unordered_set<std::string> identifiers;
  *valid = request.blocked_application_id_count > 0 && request.blocked_application_ids;
  for (uint32_t index = 0; *valid && index < request.blocked_application_id_count; ++index) {
    const auto& view = request.blocked_application_ids[index];
    if (!view.data || view.len == 0 || view.len > kMaximumIdentifierBytes) {
      *valid = false;
      break;
    }
    std::string identifier(reinterpret_cast<const char*>(view.data),
                           static_cast<size_t>(view.len));
    if (!valid_windows_identifier(identifier)) {
      *valid = false;
      break;
    }
    identifiers.insert(std::move(identifier));
  }
  return identifiers;
}

bool blocked_process(DWORD process_id,
                     const std::unordered_set<std::string>& blocked) {
  const std::wstring path = process_path(process_id);
  if (path.empty()) return false;
  const std::string identifier = application_identifier(path);
  return !identifier.empty() && blocked.contains(identifier);
}

struct WindowSearch {
  const std::unordered_set<std::string>* blocked = nullptr;
  std::vector<HWND> windows;
};

BOOL CALLBACK collect_blocked_windows(HWND window, LPARAM parameter) {
  auto* search = reinterpret_cast<WindowSearch*>(parameter);
  DWORD process_id = 0;
  GetWindowThreadProcessId(window, &process_id);
  if (process_id && blocked_process(process_id, *search->blocked)) {
    search->windows.push_back(window);
  }
  return TRUE;
}

std::vector<HWND> blocked_windows(
    const std::unordered_set<std::string>& blocked) {
  WindowSearch search{&blocked, {}};
  EnumWindows(collect_blocked_windows, reinterpret_cast<LPARAM>(&search));
  std::sort(search.windows.begin(), search.windows.end());
  return search.windows;
}

HMONITOR primary_monitor() {
  return MonitorFromPoint(POINT{0, 0}, MONITOR_DEFAULTTOPRIMARY);
}

wgc::GraphicsCaptureItem capture_item_for_monitor(HMONITOR monitor) {
  auto factory = winrt::get_activation_factory<wgc::GraphicsCaptureItem,
                                                IGraphicsCaptureItemInterop>();
  wgc::GraphicsCaptureItem item{nullptr};
  winrt::check_hresult(factory->CreateForMonitor(
      monitor, winrt::guid_of<wgc::GraphicsCaptureItem>(), winrt::put_abi(item)));
  return item;
}

wgd3d::IDirect3DDevice make_winrt_device(
    winrt::com_ptr<ID3D11Device>* native_device) {
  const D3D_FEATURE_LEVEL levels[] = {
      D3D_FEATURE_LEVEL_11_1, D3D_FEATURE_LEVEL_11_0,
      D3D_FEATURE_LEVEL_10_1, D3D_FEATURE_LEVEL_10_0};
  D3D_FEATURE_LEVEL level{};
  HRESULT hr = D3D11CreateDevice(
      nullptr, D3D_DRIVER_TYPE_HARDWARE, nullptr,
      D3D11_CREATE_DEVICE_BGRA_SUPPORT, levels, ARRAYSIZE(levels),
      D3D11_SDK_VERSION, native_device->put(), &level, nullptr);
  if (FAILED(hr)) {
    hr = D3D11CreateDevice(
        nullptr, D3D_DRIVER_TYPE_WARP, nullptr,
        D3D11_CREATE_DEVICE_BGRA_SUPPORT, levels, ARRAYSIZE(levels),
        D3D11_SDK_VERSION, native_device->put(), &level, nullptr);
  }
  winrt::check_hresult(hr);
  auto dxgi_device = native_device->as<IDXGIDevice>();
  winrt::com_ptr<IInspectable> inspectable;
  winrt::check_hresult(CreateDirect3D11DeviceFromDXGIDevice(
      dxgi_device.get(), inspectable.put()));
  return inspectable.as<wgd3d::IDirect3DDevice>();
}

wui::WindowId window_id(HWND window) {
  ABI::Windows::UI::WindowId value{};
  winrt::check_hresult(GetWindowIdFromWindow(window, &value));
  return wui::WindowId{value.Value};
}

winrt::com_ptr<ID3D11Texture2D> texture_from_frame(
    const wgc::Direct3D11CaptureFrame& frame) {
  auto access = frame.Surface().as<
      ::Windows::Graphics::DirectX::Direct3D11::IDirect3DDxgiInterfaceAccess>();
  winrt::com_ptr<ID3D11Texture2D> texture;
  winrt::check_hresult(access->GetInterface(
      winrt::guid_of<ID3D11Texture2D>(), texture.put_void()));
  return texture;
}

std::vector<uint8_t> read_scaled_pixels(ID3D11Device* device,
                                        ID3D11Texture2D* source,
                                        uint32_t target_height,
                                        uint32_t* target_width) {
  D3D11_TEXTURE2D_DESC description{};
  source->GetDesc(&description);
  D3D11_TEXTURE2D_DESC staging_description = description;
  staging_description.Usage = D3D11_USAGE_STAGING;
  staging_description.BindFlags = 0;
  staging_description.CPUAccessFlags = D3D11_CPU_ACCESS_READ;
  staging_description.MiscFlags = 0;
  winrt::com_ptr<ID3D11Texture2D> staging;
  winrt::check_hresult(device->CreateTexture2D(
      &staging_description, nullptr, staging.put()));
  winrt::com_ptr<ID3D11DeviceContext> context;
  device->GetImmediateContext(context.put());
  context->CopyResource(staging.get(), source);
  D3D11_MAPPED_SUBRESOURCE mapped{};
  winrt::check_hresult(context->Map(staging.get(), 0, D3D11_MAP_READ, 0, &mapped));
  const uint32_t width = description.Width;
  const uint32_t height = description.Height;
  *target_width = std::max<uint32_t>(
      1, static_cast<uint32_t>((static_cast<uint64_t>(width) * target_height + height / 2) / height));
  std::vector<uint8_t> pixels(static_cast<size_t>(*target_width) * target_height * 4);
  for (uint32_t y = 0; y < target_height; ++y) {
    const uint32_t source_y = std::min<uint32_t>(height - 1,
        static_cast<uint32_t>(static_cast<uint64_t>(y) * height / target_height));
    for (uint32_t x = 0; x < *target_width; ++x) {
      const uint32_t source_x = std::min<uint32_t>(width - 1,
          static_cast<uint32_t>(static_cast<uint64_t>(x) * width / *target_width));
      memcpy(pixels.data() + (static_cast<size_t>(y) * *target_width + x) * 4,
             static_cast<const uint8_t*>(mapped.pData) +
                 static_cast<size_t>(source_y) * mapped.RowPitch + source_x * 4,
             4);
    }
  }
  context->Unmap(staging.get(), 0);
  return pixels;
}

HRESULT encode_jpeg(const std::wstring& path, const std::vector<uint8_t>& pixels,
                    uint32_t width, uint32_t height, uint32_t quality) {
  winrt::com_ptr<IWICImagingFactory> factory;
  HRESULT hr = CoCreateInstance(CLSID_WICImagingFactory, nullptr,
                                CLSCTX_INPROC_SERVER, IID_PPV_ARGS(factory.put()));
  winrt::com_ptr<IWICStream> stream;
  winrt::com_ptr<IWICBitmapEncoder> encoder;
  winrt::com_ptr<IWICBitmapFrameEncode> frame;
  winrt::com_ptr<IPropertyBag2> properties;
  winrt::com_ptr<IWICBitmap> bitmap;
  winrt::com_ptr<IWICFormatConverter> converter;
  if (SUCCEEDED(hr)) hr = factory->CreateStream(stream.put());
  if (SUCCEEDED(hr)) hr = stream->InitializeFromFilename(path.c_str(), GENERIC_WRITE);
  if (SUCCEEDED(hr)) hr = factory->CreateEncoder(GUID_ContainerFormatJpeg, nullptr, encoder.put());
  if (SUCCEEDED(hr)) hr = encoder->Initialize(stream.get(), WICBitmapEncoderNoCache);
  if (SUCCEEDED(hr)) hr = encoder->CreateNewFrame(frame.put(), properties.put());
  if (SUCCEEDED(hr) && properties) {
    PROPBAG2 option{};
    option.pstrName = const_cast<LPOLESTR>(L"ImageQuality");
    VARIANT value;
    VariantInit(&value);
    value.vt = VT_R4;
    value.fltVal = static_cast<float>(quality) / 100.0f;
    properties->Write(1, &option, &value);
    VariantClear(&value);
  }
  if (SUCCEEDED(hr)) hr = frame->Initialize(properties.get());
  if (SUCCEEDED(hr)) hr = frame->SetSize(width, height);
  WICPixelFormatGUID format = GUID_WICPixelFormat24bppBGR;
  if (SUCCEEDED(hr)) hr = frame->SetPixelFormat(&format);
  if (SUCCEEDED(hr)) hr = factory->CreateBitmapFromMemory(
      width, height, GUID_WICPixelFormat32bppBGRA, width * 4,
      static_cast<UINT>(pixels.size()), const_cast<BYTE*>(pixels.data()), bitmap.put());
  if (SUCCEEDED(hr)) hr = factory->CreateFormatConverter(converter.put());
  if (SUCCEEDED(hr)) hr = converter->Initialize(
      bitmap.get(), GUID_WICPixelFormat24bppBGR, WICBitmapDitherTypeNone,
      nullptr, 0.0, WICBitmapPaletteTypeCustom);
  if (SUCCEEDED(hr)) hr = frame->WriteSource(converter.get(), nullptr);
  if (SUCCEEDED(hr)) hr = frame->Commit();
  if (SUCCEEDED(hr)) hr = encoder->Commit();
  return hr;
}

std::wstring temporary_path(const std::wstring& output) {
  return output + L".daygo-" + std::to_wstring(GetCurrentProcessId()) + L"-" +
         std::to_wstring(GetTickCount64()) + L".partial";
}

int32_t capture_failure(int32_t status, HRESULT native,
                        dg_capture_error_v1* error) {
  if (error && error->struct_size >= sizeof(*error)) {
    error->native_domain = native == S_OK ? DG_CAPTURE_NATIVE_NONE
                                          : DG_CAPTURE_NATIVE_WINDOWS;
    error->native_code = native;
  }
  return status;
}

int64_t unix_time_ns() {
  FILETIME now{};
  GetSystemTimeAsFileTime(&now);
  ULARGE_INTEGER ticks{now.dwLowDateTime, now.dwHighDateTime};
  return static_cast<int64_t>((ticks.QuadPart - 116444736000000000ULL) * 100ULL);
}

}  // namespace

extern "C" {

__declspec(dllexport) void DG_APPLICATION_CALL dg_application_abi_version(
    uint32_t* major, uint32_t* minor) {
  if (major) *major = DG_APPLICATION_ABI_MAJOR;
  if (minor) *minor = DG_APPLICATION_ABI_MINOR;
}

__declspec(dllexport) int32_t DG_APPLICATION_CALL dg_application_inspect(
    uint32_t requested_abi_major,
    dg_application_string_view_v1 application_path,
    dg_application_info_v2* out_info,
    dg_application_error_v1* out_error) {
  if (!out_info) return DG_APPLICATION_E_INVALID_ARGUMENT;
  if (!valid_application_call(requested_abi_major, out_info, out_error)) {
    clear_application_info(out_info);
    return application_failure(
        requested_abi_major == DG_APPLICATION_ABI_MAJOR
            ? DG_APPLICATION_E_INVALID_ARGUMENT
            : DG_APPLICATION_E_ABI_MISMATCH,
        S_OK, out_error);
  }
  std::wstring path;
  if (!utf8_to_wide(application_path.data, application_path.len,
                    kMaximumPathBytes, &path)) {
    return application_failure(DG_APPLICATION_E_INVALID_ARGUMENT, S_OK, out_error);
  }
  ApplicationData application;
  if (!inspect_path(path, &application)) {
    return application_failure(DG_APPLICATION_E_NOT_APPLICATION,
                               HRESULT_FROM_WIN32(GetLastError()), out_error);
  }
  {
    std::lock_guard lock(application_cache_mutex);
    application_cache[application.identifier] = application;
  }
  return write_application(application, out_info, out_error);
}

__declspec(dllexport) int32_t DG_APPLICATION_CALL dg_application_lookup(
    uint32_t requested_abi_major,
    dg_application_string_view_v1 identifier_view,
    dg_application_info_v2* out_info,
    dg_application_error_v1* out_error) {
  if (!out_info) return DG_APPLICATION_E_INVALID_ARGUMENT;
  if (!valid_application_call(requested_abi_major, out_info, out_error)) {
    clear_application_info(out_info);
    return application_failure(
        requested_abi_major == DG_APPLICATION_ABI_MAJOR
            ? DG_APPLICATION_E_INVALID_ARGUMENT
            : DG_APPLICATION_E_ABI_MISMATCH,
        S_OK, out_error);
  }
  if (!identifier_view.data || identifier_view.len == 0 ||
      identifier_view.len > kMaximumIdentifierBytes) {
    return application_failure(DG_APPLICATION_E_INVALID_ARGUMENT, S_OK, out_error);
  }
  std::string identifier(reinterpret_cast<const char*>(identifier_view.data),
                         static_cast<size_t>(identifier_view.len));
  if (!valid_windows_identifier(identifier)) {
    return application_failure(DG_APPLICATION_E_INVALID_ARGUMENT, S_OK, out_error);
  }
  ApplicationData application;
  if (!find_application_by_identifier(identifier, &application)) {
    return application_failure(DG_APPLICATION_E_NOT_FOUND,
                               HRESULT_FROM_WIN32(ERROR_NOT_FOUND), out_error);
  }
  return write_application(application, out_info, out_error);
}

__declspec(dllexport) uint32_t DG_CAPTURE_CALL dg_windows_privacy_build() {
  return windows_build_number();
}

__declspec(dllexport) int32_t DG_CAPTURE_CALL dg_windows_wgc_capture_once(
    uint32_t requested_abi_major, const dg_capture_request_v1* request,
    dg_capture_result_v1* result, dg_capture_error_v1* error) {
  if (!request || !result) return DG_CAPTURE_E_INVALID_ARGUMENT;
  const uint32_t result_size = result->struct_size;
  result->image_format = 0;
  result->captured_at_unix_ns = 0;
  result->file_size = 0;
  result->width = 0;
  result->height = 0;
  if (error && error->struct_size >= sizeof(*error)) {
    error->native_domain = DG_CAPTURE_NATIVE_NONE;
    error->native_code = 0;
  }
  if (requested_abi_major != DG_CAPTURE_ABI_MAJOR ||
      request->struct_size < sizeof(*request) || result_size < sizeof(*result) ||
      (error && error->struct_size < sizeof(*error))) {
    return capture_failure(DG_CAPTURE_E_ABI_MISMATCH, S_OK, error);
  }
  if (windows_build_number() < kMinimumPrivacyBuild) {
    return capture_failure(DG_CAPTURE_E_PRIVACY_UNSUPPORTED, E_NOTIMPL, error);
  }
  bool identifiers_valid = false;
  const auto blocked = decode_blocked_identifiers(*request, &identifiers_valid);
  if (!identifiers_valid || request->target_height == 0 ||
      request->jpeg_quality == 0 || request->jpeg_quality > 100 ||
      request->timeout_ms == 0) {
    return capture_failure(DG_CAPTURE_E_INVALID_ARGUMENT, S_OK, error);
  }
  std::wstring output;
  if (!utf8_to_wide(request->output_path.data, request->output_path.len,
                    kMaximumPathBytes, &output)) {
    return capture_failure(DG_CAPTURE_E_INVALID_ARGUMENT, S_OK, error);
  }
  HWND foreground = GetForegroundWindow();
  DWORD foreground_process = 0;
  if (foreground) GetWindowThreadProcessId(foreground, &foreground_process);
  if (foreground_process && blocked_process(foreground_process, blocked)) {
    return DG_CAPTURE_BLOCKED;
  }

  const std::vector<HWND> before_windows = blocked_windows(blocked);
  try {
    Apartment apartment;
    if (!apartment.usable() || !wgc::GraphicsCaptureSession::IsSupported()) {
      return capture_failure(DG_CAPTURE_E_PRIVACY_UNSUPPORTED,
                             apartment.usable() ? E_NOTIMPL : apartment.result, error);
    }
    auto item = capture_item_for_monitor(primary_monitor());
    winrt::com_ptr<ID3D11Device> native_device;
    auto winrt_device = make_winrt_device(&native_device);
    auto pool = wgc::Direct3D11CaptureFramePool::CreateFreeThreaded(
        winrt_device, wgd::DirectXPixelFormat::B8G8R8A8UIntNormalized,
        2, item.Size());
    auto session = pool.CreateCaptureSession(item);
    session.IsCursorCaptureEnabled((request->flags & DG_CAPTURE_SHOWS_CURSOR) != 0);
    auto display_session = session.try_as<wgc::IDisplayGraphicsCaptureSession>();
    if (!display_session) {
      session.Close();
      pool.Close();
      return capture_failure(DG_CAPTURE_E_PRIVACY_UNSUPPORTED, E_NOINTERFACE, error);
    }
    std::vector<wui::WindowId> ids;
    ids.reserve(before_windows.size());
    for (HWND window : before_windows) ids.push_back(window_id(window));
    const auto iterable = winrt::single_threaded_vector<wui::WindowId>(std::move(ids));
    const uint64_t required_iteration = display_session.SetWindowExclusionList(iterable);

    std::mutex mutex;
    std::condition_variable ready;
    wgc::Direct3D11CaptureFrame selected{nullptr};
    std::exception_ptr callback_error;
    const auto token = pool.FrameArrived(
        [&](const wgc::Direct3D11CaptureFramePool& sender,
            const winrt::Windows::Foundation::IInspectable&) {
          try {
            auto frame = sender.TryGetNextFrame();
            if (!frame) return;
            auto versioned = frame.try_as<wgc::IDirect3D11CaptureFrame3>();
            if (!versioned || versioned.ConfigurationIteration() < required_iteration) return;
            {
              std::lock_guard lock(mutex);
              if (!selected) selected = frame;
            }
            ready.notify_one();
          } catch (...) {
            {
              std::lock_guard lock(mutex);
              callback_error = std::current_exception();
            }
            ready.notify_one();
          }
        });
    session.StartCapture();
    const auto timeout = std::chrono::milliseconds(
        std::min<uint32_t>(request->timeout_ms, 60000));
    {
      std::unique_lock lock(mutex);
      if (!ready.wait_for(lock, timeout,
                          [&] { return selected || callback_error; })) {
        pool.FrameArrived(token);
        session.Close();
        pool.Close();
        return capture_failure(DG_CAPTURE_E_TIMEOUT,
                               HRESULT_FROM_WIN32(WAIT_TIMEOUT), error);
      }
    }
    pool.FrameArrived(token);
    session.Close();
    if (callback_error) std::rethrow_exception(callback_error);
    if (!selected) {
      pool.Close();
      return capture_failure(DG_CAPTURE_E_INTERNAL, E_FAIL, error);
    }
    auto texture = texture_from_frame(selected);
    uint32_t width = 0;
    auto pixels = read_scaled_pixels(native_device.get(), texture.get(),
                                     request->target_height, &width);
    selected.Close();
    pool.Close();

    if (before_windows != blocked_windows(blocked)) {
      return DG_CAPTURE_BLOCKED;
    }
    const std::wstring temporary = temporary_path(output);
    DeleteFileW(temporary.c_str());
    HRESULT hr = encode_jpeg(temporary, pixels, width, request->target_height,
                             request->jpeg_quality);
    if (SUCCEEDED(hr) && !MoveFileExW(temporary.c_str(), output.c_str(),
                                     MOVEFILE_WRITE_THROUGH)) {
      hr = HRESULT_FROM_WIN32(GetLastError());
    }
    if (FAILED(hr)) {
      DeleteFileW(temporary.c_str());
      return capture_failure(DG_CAPTURE_E_IO, hr, error);
    }
    WIN32_FILE_ATTRIBUTE_DATA attributes{};
    if (!GetFileAttributesExW(output.c_str(), GetFileExInfoStandard, &attributes)) {
      DeleteFileW(output.c_str());
      return capture_failure(DG_CAPTURE_E_IO,
                             HRESULT_FROM_WIN32(GetLastError()), error);
    }
    ULARGE_INTEGER file_size{attributes.nFileSizeLow, attributes.nFileSizeHigh};
    result->image_format = DG_CAPTURE_IMAGE_JPEG;
    result->captured_at_unix_ns = unix_time_ns();
    result->file_size = file_size.QuadPart;
    result->width = width;
    result->height = request->target_height;
    return DG_CAPTURE_OK;
  } catch (const winrt::hresult_error& failure) {
    return capture_failure(DG_CAPTURE_E_PRIVACY_UNSUPPORTED,
                           failure.code(), error);
  } catch (...) {
    return capture_failure(DG_CAPTURE_E_INTERNAL, E_FAIL, error);
  }
}

}  // extern "C"
