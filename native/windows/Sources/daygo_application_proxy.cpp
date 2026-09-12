#define DAYGO_APPLICATION_BUILD 1
#include "../../include/daygo_application.h"

#include <windows.h>

#include <iterator>
#include <cstring>
#include <string>

namespace {

constexpr wchar_t kNativeDLLName[] = L"daygo_windows_native.dll";

HMODULE load_native_dll() {
  static HMODULE module = []() -> HMODULE {
    wchar_t executable[32768] = {};
    const DWORD executable_size = GetModuleFileNameW(
        nullptr, executable, static_cast<DWORD>(std::size(executable)));
    if (executable_size > 0 && executable_size < std::size(executable)) {
      std::wstring sibling(executable, executable_size);
      const size_t slash = sibling.find_last_of(L"\\/");
      if (slash != std::wstring::npos) {
        sibling.resize(slash + 1);
        sibling += kNativeDLLName;
        if (HMODULE value = LoadLibraryW(sibling.c_str())) return value;
      }
    }
    return LoadLibraryExW(kNativeDLLName, nullptr, LOAD_LIBRARY_SEARCH_DEFAULT_DIRS);
  }();
  return module;
}

template <typename Function>
Function resolve(const char* name) {
  HMODULE module = load_native_dll();
  FARPROC procedure = module ? GetProcAddress(module, name) : nullptr;
  Function function = nullptr;
  static_assert(sizeof(function) == sizeof(procedure), "unexpected function pointer size");
  std::memcpy(&function, &procedure, sizeof(function));
  return function;
}

void set_unavailable(dg_application_error_v1* error) {
  if (error && error->struct_size >= sizeof(*error)) {
    error->native_domain = DG_APPLICATION_NATIVE_WINDOWS;
    error->native_code = HRESULT_FROM_WIN32(GetLastError() == ERROR_SUCCESS
                                                ? ERROR_MOD_NOT_FOUND
                                                : GetLastError());
  }
}

}  // namespace

extern "C" {

DG_APPLICATION_API void DG_APPLICATION_CALL dg_application_abi_version(
    uint32_t* major, uint32_t* minor) {
  using Function = void(DG_APPLICATION_CALL*)(uint32_t*, uint32_t*);
  if (Function function = resolve<Function>("dg_application_abi_version")) {
    function(major, minor);
    return;
  }
  if (major) *major = 0;
  if (minor) *minor = 0;
}

DG_APPLICATION_API int32_t DG_APPLICATION_CALL dg_application_inspect(
    uint32_t requested_abi_major,
    dg_application_string_view_v1 application_path,
    dg_application_info_v2* out_info,
    dg_application_error_v1* out_error) {
  using Function = int32_t(DG_APPLICATION_CALL*)(
      uint32_t, dg_application_string_view_v1, dg_application_info_v2*,
      dg_application_error_v1*);
  if (Function function = resolve<Function>("dg_application_inspect")) {
    return function(requested_abi_major, application_path, out_info, out_error);
  }
  set_unavailable(out_error);
  return DG_APPLICATION_E_UNSUPPORTED;
}

DG_APPLICATION_API int32_t DG_APPLICATION_CALL dg_application_lookup(
    uint32_t requested_abi_major,
    dg_application_string_view_v1 identifier,
    dg_application_info_v2* out_info,
    dg_application_error_v1* out_error) {
  using Function = int32_t(DG_APPLICATION_CALL*)(
      uint32_t, dg_application_string_view_v1, dg_application_info_v2*,
      dg_application_error_v1*);
  if (Function function = resolve<Function>("dg_application_lookup")) {
    return function(requested_abi_major, identifier, out_info, out_error);
  }
  set_unavailable(out_error);
  return DG_APPLICATION_E_UNSUPPORTED;
}

}  // extern "C"
