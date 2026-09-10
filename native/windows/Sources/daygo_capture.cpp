#define DAYGO_CAPTURE_BUILD 1
#define _WIN32_WINNT 0x0602
#include "../../include/daygo_capture.h"

#include <windows.h>
#include <d3d11.h>
#include <dxgi1_2.h>
#include <wincodec.h>

#include <algorithm>
#include <cstdint>
#include <cstdio>
#include <cstring>
#include <string>
#include <vector>

namespace {

constexpr int kMaxPathBytes = 32768;
constexpr int kMaxApplicationIDBytes = 4096;
constexpr int kMaxApplicationIDs = 4096;
constexpr int kMaxApplicationIDTotalBytes = 1 << 20;

template <typename T>
class ComPtr {
 public:
  ComPtr() = default;
  ~ComPtr() { reset(); }
  ComPtr(const ComPtr&) = delete;
  ComPtr& operator=(const ComPtr&) = delete;
  T* get() const { return value_; }
  T* operator->() const { return value_; }
  T** put() {
    reset();
    return &value_;
  }
  void reset(T* value = nullptr) {
    if (value_) value_->Release();
    value_ = value;
  }

 private:
  T* value_ = nullptr;
};

struct ComScope {
  HRESULT result;
  ComScope() : result(CoInitializeEx(nullptr, COINIT_MULTITHREADED)) {}
  ~ComScope() {
    if (SUCCEEDED(result)) CoUninitialize();
  }
  bool usable() const { return SUCCEEDED(result) || result == RPC_E_CHANGED_MODE; }
};

void debug_stage(const char* stage) {
  char enabled[2] = {};
  DWORD n = GetEnvironmentVariableA("DAYGO_CAPTURE_DEBUG", enabled, sizeof(enabled));
  if (n == 1 && enabled[0] == '1') {
    std::fprintf(stderr, "[daygo.capture] stage=%s\n", stage);
  }
}

int32_t fail(int32_t status, HRESULT native_code, dg_capture_error_v1* error) {
  if (error) {
    error->native_domain = native_code == S_OK ? DG_CAPTURE_NATIVE_NONE : DG_CAPTURE_NATIVE_WINDOWS;
    error->native_code = static_cast<int64_t>(native_code);
  }
  return status;
}

bool decode_utf8(const dg_capture_string_view_v1& view, int maximum, std::string* out) {
  if (view.len == 0 || view.len > static_cast<uint64_t>(maximum) || view.data == nullptr) return false;
  out->assign(reinterpret_cast<const char*>(view.data), static_cast<size_t>(view.len));
  int required = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, out->data(), static_cast<int>(out->size()), nullptr, 0);
  return required > 0;
}

bool utf8_to_wide(const std::string& input, std::wstring* output) {
  int count = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, input.data(), static_cast<int>(input.size()), nullptr, 0);
  if (count <= 0) return false;
  output->resize(static_cast<size_t>(count));
  return MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, input.data(), static_cast<int>(input.size()), output->data(), count) == count;
}

bool is_absolute_windows_path(const std::wstring& path) {
  if (path.size() >= 3 && ((path[0] >= L'a' && path[0] <= L'z') || (path[0] >= L'A' && path[0] <= L'Z')) && path[1] == L':' && (path[2] == L'\\' || path[2] == L'/')) return true;
  return path.size() >= 2 && path[0] == L'\\' && path[1] == L'\\';
}

bool valid_extension(const std::wstring& path) {
  const size_t slash = path.find_last_of(L"\\/");
  const size_t dot = path.find_last_of(L'.');
  if (dot == std::wstring::npos || (slash != std::wstring::npos && dot < slash)) return false;
  std::wstring ext = path.substr(dot);
  std::transform(ext.begin(), ext.end(), ext.begin(), [](wchar_t c) { return static_cast<wchar_t>(towlower(c)); });
  return ext == L".jpg" || ext == L".jpeg";
}

struct DecodedRequest {
  std::wstring output_path;
  uint32_t target_height = 0;
  uint32_t jpeg_quality = 0;
  uint32_t timeout_ms = 0;
  bool shows_cursor = false;
  std::vector<std::string> blocked_ids;
};

HRESULT frontmost_application_id(std::string* identifier) {
  HWND window = GetForegroundWindow();
  if (!window) return HRESULT_FROM_WIN32(ERROR_NOT_FOUND);
  DWORD process_id = 0;
  if (!GetWindowThreadProcessId(window, &process_id) || process_id == 0) return HRESULT_FROM_WIN32(ERROR_NOT_FOUND);
  HANDLE process = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, process_id);
  if (!process) return HRESULT_FROM_WIN32(GetLastError());
  using GetApplicationUserModelIdFn = LONG(WINAPI*)(HANDLE, UINT32*, PWSTR);
  FARPROC proc = GetProcAddress(GetModuleHandleW(L"kernel32.dll"), "GetApplicationUserModelId");
  GetApplicationUserModelIdFn get_application_user_model_id = nullptr;
  static_assert(sizeof(get_application_user_model_id) == sizeof(proc), "unexpected function pointer size");
  std::memcpy(&get_application_user_model_id, &proc, sizeof(proc));
  if (!get_application_user_model_id) {
    CloseHandle(process);
    return E_NOTIMPL;
  }
  UINT32 length = 0;
  LONG result = get_application_user_model_id(process, &length, nullptr);
  if (result != ERROR_INSUFFICIENT_BUFFER || length == 0 || length > 4096) {
    CloseHandle(process);
    return HRESULT_FROM_WIN32(result == ERROR_SUCCESS ? ERROR_NOT_FOUND : result);
  }
  std::wstring value(length, L'\0');
  result = get_application_user_model_id(process, &length, value.data());
  CloseHandle(process);
  if (result != ERROR_SUCCESS || length <= 1) return HRESULT_FROM_WIN32(result == ERROR_SUCCESS ? ERROR_NOT_FOUND : result);
  value.resize(length - 1);
  int bytes = WideCharToMultiByte(CP_UTF8, 0, value.data(), static_cast<int>(value.size()), nullptr, 0, nullptr, nullptr);
  if (bytes <= 0) return HRESULT_FROM_WIN32(GetLastError());
  identifier->resize(static_cast<size_t>(bytes));
  if (WideCharToMultiByte(CP_UTF8, 0, value.data(), static_cast<int>(value.size()), identifier->data(), bytes, nullptr, nullptr) != bytes) return HRESULT_FROM_WIN32(GetLastError());
  return S_OK;
}

bool decode_request(const dg_capture_request_v1& request, DecodedRequest* decoded) {
  if ((request.flags & ~static_cast<uint32_t>(DG_CAPTURE_SHOWS_CURSOR)) != 0 ||
      request.image_format != DG_CAPTURE_IMAGE_JPEG || request.target_height < 1 ||
      request.target_height > 16384 || request.jpeg_quality < 1 || request.jpeg_quality > 100 ||
      request.timeout_ms < 1 || request.timeout_ms > 60000 || request.blocked_application_id_count > kMaxApplicationIDs ||
      request.reserved0 != 0) return false;

  std::string output_utf8;
  if (!decode_utf8(request.output_path, kMaxPathBytes, &output_utf8) || !utf8_to_wide(output_utf8, &decoded->output_path) ||
      !is_absolute_windows_path(decoded->output_path) || !valid_extension(decoded->output_path)) return false;

  uint64_t total = 0;
  if (request.blocked_application_id_count != 0 && request.blocked_application_ids == nullptr) return false;
  for (uint32_t i = 0; i < request.blocked_application_id_count; ++i) {
    std::string id;
    if (!decode_utf8(request.blocked_application_ids[i], kMaxApplicationIDBytes, &id)) return false;
    total += id.size();
    if (total > kMaxApplicationIDTotalBytes) return false;
    decoded->blocked_ids.push_back(std::move(id));
  }
  decoded->target_height = request.target_height;
  decoded->jpeg_quality = request.jpeg_quality;
  decoded->timeout_ms = request.timeout_ms;
  decoded->shows_cursor = (request.flags & DG_CAPTURE_SHOWS_CURSOR) != 0;
  return true;
}

struct PrimaryMonitorContext {
  HMONITOR monitor = nullptr;
};

BOOL CALLBACK find_primary_monitor(HMONITOR monitor, HDC, LPRECT, LPARAM data) {
  MONITORINFO info{};
  info.cbSize = sizeof(info);
  if (GetMonitorInfoW(monitor, &info) && (info.dwFlags & MONITORINFOF_PRIMARY)) {
    static_cast<PrimaryMonitorContext*>(reinterpret_cast<void*>(data))->monitor = monitor;
    return FALSE;
  }
  return TRUE;
}

HMONITOR primary_monitor() {
  PrimaryMonitorContext context;
  EnumDisplayMonitors(nullptr, nullptr, find_primary_monitor, reinterpret_cast<LPARAM>(&context));
  return context.monitor;
}

HRESULT find_output(HMONITOR monitor, ComPtr<IDXGIAdapter1>* adapter_out, ComPtr<IDXGIOutput1>* output_out) {
  ComPtr<IDXGIFactory1> factory;
  HRESULT hr = CreateDXGIFactory1(IID_PPV_ARGS(factory.put()));
  if (FAILED(hr)) return hr;
  for (UINT ai = 0;; ++ai) {
    ComPtr<IDXGIAdapter1> adapter;
    hr = factory->EnumAdapters1(ai, adapter.put());
    if (hr == DXGI_ERROR_NOT_FOUND) break;
    if (FAILED(hr)) return hr;
    DXGI_ADAPTER_DESC1 adapter_desc{};
    if (FAILED(adapter->GetDesc1(&adapter_desc)) || (adapter_desc.Flags & 0x2u)) continue;
    for (UINT oi = 0;; ++oi) {
      ComPtr<IDXGIOutput> output;
      hr = adapter->EnumOutputs(oi, output.put());
      if (hr == DXGI_ERROR_NOT_FOUND) break;
      if (FAILED(hr)) return hr;
      DXGI_OUTPUT_DESC desc{};
      if (FAILED(output->GetDesc(&desc)) || desc.Monitor != monitor) continue;
      hr = output->QueryInterface(IID_PPV_ARGS(output_out->put()));
      if (SUCCEEDED(hr)) adapter_out->reset(adapter.get()), adapter->AddRef();
      return hr;
    }
  }
  return DXGI_ERROR_NOT_FOUND;
}

void rotate_bgra(const std::vector<uint8_t>& source, UINT width, UINT height, DXGI_MODE_ROTATION rotation,
                 std::vector<uint8_t>* destination, UINT* out_width, UINT* out_height) {
  if (rotation == DXGI_MODE_ROTATION_IDENTITY || rotation == DXGI_MODE_ROTATION_UNSPECIFIED) {
    *destination = source;
    *out_width = width;
    *out_height = height;
    return;
  }
  const bool quarter_turn = rotation == DXGI_MODE_ROTATION_ROTATE90 || rotation == DXGI_MODE_ROTATION_ROTATE270;
  *out_width = quarter_turn ? height : width;
  *out_height = quarter_turn ? width : height;
  destination->assign(static_cast<size_t>(*out_width) * *out_height * 4, 0);
  for (UINT y = 0; y < height; ++y) {
    for (UINT x = 0; x < width; ++x) {
      UINT dx = x, dy = y;
      switch (rotation) {
        case DXGI_MODE_ROTATION_ROTATE90: dx = height - 1 - y; dy = x; break;
        case DXGI_MODE_ROTATION_ROTATE180: dx = width - 1 - x; dy = height - 1 - y; break;
        case DXGI_MODE_ROTATION_ROTATE270: dx = y; dy = width - 1 - x; break;
        default: break;
      }
      const uint8_t* src = source.data() + (static_cast<size_t>(y) * width + x) * 4;
      uint8_t* dst = destination->data() + (static_cast<size_t>(dy) * *out_width + dx) * 4;
      std::copy(src, src + 4, dst);
    }
  }
}

std::vector<uint8_t> scale_bgra(const std::vector<uint8_t>& source, UINT width, UINT height, UINT target_height, UINT* out_width) {
  *out_width = std::max<UINT>(1, static_cast<UINT>((static_cast<double>(width) * target_height / height) + 0.5));
  std::vector<uint8_t> destination(static_cast<size_t>(*out_width) * target_height * 4);
  for (UINT y = 0; y < target_height; ++y) {
    UINT sy = std::min(height - 1, static_cast<UINT>((static_cast<uint64_t>(y) * height) / target_height));
    for (UINT x = 0; x < *out_width; ++x) {
      UINT sx = std::min(width - 1, static_cast<UINT>((static_cast<uint64_t>(x) * width) / *out_width));
      const uint8_t* src = source.data() + (static_cast<size_t>(sy) * width + sx) * 4;
      uint8_t* dst = destination.data() + (static_cast<size_t>(y) * *out_width + x) * 4;
      std::copy(src, src + 4, dst);
    }
  }
  return destination;
}

HRESULT make_temp_path(const std::wstring& output, std::wstring* temp) {
  const size_t slash = output.find_last_of(L"\\/");
  const std::wstring parent = slash == std::wstring::npos ? L"." : output.substr(0, slash);
  const std::wstring stem = slash == std::wstring::npos ? output : output.substr(slash + 1);
  for (uint32_t attempt = 0; attempt < 100; ++attempt) {
    std::wstring candidate = parent + L"\\." + stem + L".daygo-" + std::to_wstring(GetCurrentProcessId()) + L"-" + std::to_wstring(static_cast<uint64_t>(GetTickCount()) + attempt) + L".partial";
    HANDLE handle = CreateFileW(candidate.c_str(), GENERIC_WRITE, 0, nullptr, CREATE_NEW, FILE_ATTRIBUTE_NORMAL, nullptr);
    if (handle != INVALID_HANDLE_VALUE) {
      CloseHandle(handle);
      *temp = std::move(candidate);
      return S_OK;
    }
    if (GetLastError() != ERROR_FILE_EXISTS && GetLastError() != ERROR_ALREADY_EXISTS) return HRESULT_FROM_WIN32(GetLastError());
  }
  return HRESULT_FROM_WIN32(ERROR_FILE_EXISTS);
}

HRESULT encode_jpeg(const std::wstring& path, const std::vector<uint8_t>& pixels, UINT width, UINT height, UINT quality) {
  ComPtr<IWICImagingFactory> factory;
  HRESULT hr = CoCreateInstance(CLSID_WICImagingFactory, nullptr, CLSCTX_INPROC_SERVER, IID_PPV_ARGS(factory.put()));
  if (FAILED(hr)) return hr;
  ComPtr<IWICStream> stream;
  hr = factory->CreateStream(stream.put());
  if (FAILED(hr)) return hr;
  hr = stream->InitializeFromFilename(path.c_str(), GENERIC_WRITE);
  if (FAILED(hr)) return hr;
  ComPtr<IWICBitmapEncoder> encoder;
  hr = factory->CreateEncoder(GUID_ContainerFormatJpeg, nullptr, encoder.put());
  if (FAILED(hr)) return hr;
  hr = encoder->Initialize(stream.get(), WICBitmapEncoderNoCache);
  if (FAILED(hr)) return hr;
  ComPtr<IWICBitmapFrameEncode> frame;
  ComPtr<IPropertyBag2> properties;
  hr = encoder->CreateNewFrame(frame.put(), properties.put());
  if (FAILED(hr)) return hr;
  if (properties.get()) {
    PROPBAG2 option{};
    option.pstrName = const_cast<LPOLESTR>(L"ImageQuality");
    VARIANT value;
    VariantInit(&value);
    value.vt = VT_R4;
    value.fltVal = static_cast<float>(quality) / 100.0f;
    properties->Write(1, &option, &value);
    VariantClear(&value);
  }
  hr = frame->Initialize(properties.get());
  if (FAILED(hr)) return hr;
  hr = frame->SetSize(width, height);
  if (FAILED(hr)) return hr;
  WICPixelFormatGUID format = GUID_WICPixelFormat24bppBGR;
  hr = frame->SetPixelFormat(&format);
  if (FAILED(hr)) return hr;
  ComPtr<IWICBitmap> bitmap;
  hr = factory->CreateBitmapFromMemory(width, height, GUID_WICPixelFormat32bppBGRA, width * 4,
                                       static_cast<UINT>(pixels.size()), const_cast<BYTE*>(pixels.data()), bitmap.put());
  if (FAILED(hr)) return hr;
  ComPtr<IWICFormatConverter> converter;
  hr = factory->CreateFormatConverter(converter.put());
  if (FAILED(hr)) return hr;
  hr = converter->Initialize(bitmap.get(), GUID_WICPixelFormat24bppBGR, WICBitmapDitherTypeNone, nullptr, 0.0, WICBitmapPaletteTypeCustom);
  if (FAILED(hr)) return hr;
  hr = frame->WriteSource(converter.get(), nullptr);
  if (FAILED(hr)) return hr;
  hr = frame->Commit();
  if (FAILED(hr)) return hr;
  return encoder->Commit();
}

bool capture_gdi_primary(std::vector<uint8_t>* pixels, UINT* width, UINT* height) {
  HMONITOR monitor = primary_monitor();
  if (!monitor) return false;
  MONITORINFO monitor_info{};
  monitor_info.cbSize = sizeof(monitor_info);
  if (!GetMonitorInfoW(monitor, &monitor_info)) return false;
  const LONG left = monitor_info.rcMonitor.left;
  const LONG top = monitor_info.rcMonitor.top;
  const LONG monitor_width = monitor_info.rcMonitor.right - left;
  const LONG monitor_height = monitor_info.rcMonitor.bottom - top;
  if (monitor_width <= 0 || monitor_height <= 0) return false;
  HDC screen = GetDC(nullptr);
  if (!screen) return false;
  HDC memory = CreateCompatibleDC(screen);
  if (!memory) {
    ReleaseDC(nullptr, screen);
    return false;
  }
  BITMAPINFO bitmap_info{};
  bitmap_info.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
  bitmap_info.bmiHeader.biWidth = monitor_width;
  bitmap_info.bmiHeader.biHeight = -monitor_height;
  bitmap_info.bmiHeader.biPlanes = 1;
  bitmap_info.bmiHeader.biBitCount = 32;
  bitmap_info.bmiHeader.biCompression = BI_RGB;
  void* bits = nullptr;
  HBITMAP bitmap = CreateDIBSection(screen, &bitmap_info, DIB_RGB_COLORS, &bits, nullptr, 0);
  if (!bitmap || !bits) {
    if (bitmap) DeleteObject(bitmap);
    DeleteDC(memory);
    ReleaseDC(nullptr, screen);
    return false;
  }
  HGDIOBJ old_bitmap = SelectObject(memory, bitmap);
  const BOOL copied = BitBlt(memory, 0, 0, monitor_width, monitor_height, screen, left, top, SRCCOPY | CAPTUREBLT);
  if (copied) {
    *width = static_cast<UINT>(monitor_width);
    *height = static_cast<UINT>(monitor_height);
    pixels->assign(static_cast<uint8_t*>(bits), static_cast<uint8_t*>(bits) +
                                                   static_cast<size_t>(monitor_width) * monitor_height * 4);
  }
  SelectObject(memory, old_bitmap);
  DeleteObject(bitmap);
  DeleteDC(memory);
  ReleaseDC(nullptr, screen);
  return copied == TRUE;
}

HRESULT capture_pixels(const DecodedRequest& request, std::vector<uint8_t>* pixels, UINT* width, UINT* height, int64_t* captured_ns) {
  HMONITOR monitor = primary_monitor();
  if (!monitor) return HRESULT_FROM_WIN32(ERROR_NOT_FOUND);
  ComPtr<IDXGIAdapter1> adapter;
  ComPtr<IDXGIOutput1> output;
  HRESULT hr = find_output(monitor, &adapter, &output);
  if (FAILED(hr)) return hr;
  ComPtr<ID3D11Device> device;
  ComPtr<ID3D11DeviceContext> context;
  D3D_FEATURE_LEVEL level{};
  const D3D_FEATURE_LEVEL levels[] = {D3D_FEATURE_LEVEL_11_1, D3D_FEATURE_LEVEL_11_0};
  hr = D3D11CreateDevice(adapter.get(), D3D_DRIVER_TYPE_UNKNOWN, nullptr, D3D11_CREATE_DEVICE_BGRA_SUPPORT,
                         levels, ARRAYSIZE(levels), D3D11_SDK_VERSION, device.put(), &level, context.put());
  if (FAILED(hr)) return hr;
  ComPtr<IDXGIOutputDuplication> duplication;
  hr = output->DuplicateOutput(device.get(), duplication.put());
  if (FAILED(hr)) return hr;
  DXGI_OUTDUPL_FRAME_INFO frame_info{};
  ComPtr<IDXGIResource> resource;
  debug_stage("duplication.acquire.begin");
  hr = duplication->AcquireNextFrame(request.timeout_ms, &frame_info, resource.put());
  if (FAILED(hr)) return hr;
  if (GetEnvironmentVariableA("DAYGO_CAPTURE_DEBUG", nullptr, 0) > 0) {
    char message[256];
    std::snprintf(message, sizeof(message),
                  "[daygo.capture] frame accumulated=%u present=%lld mouse=%lld pointer_visible=%d\n",
                  frame_info.AccumulatedFrames, static_cast<long long>(frame_info.LastPresentTime.QuadPart),
                  static_cast<long long>(frame_info.LastMouseUpdateTime.QuadPart), frame_info.PointerPosition.Visible);
    std::fputs(message, stderr);
  }
  ComPtr<ID3D11Texture2D> texture;
  hr = resource->QueryInterface(IID_PPV_ARGS(texture.put()));
  if (FAILED(hr)) {
    duplication->ReleaseFrame();
    return hr;
  }
  D3D11_TEXTURE2D_DESC desc{};
  texture->GetDesc(&desc);
  const bool debug_enabled = GetEnvironmentVariableA("DAYGO_CAPTURE_DEBUG", nullptr, 0) > 0;
  if (debug_enabled) {
    char message[256];
    std::snprintf(message, sizeof(message),
                  "[daygo.capture] texture format=%u size=%ux%u mip=%u array=%u sample=%u/%u\n",
                  static_cast<unsigned>(desc.Format), desc.Width, desc.Height, desc.MipLevels,
                  desc.ArraySize, desc.SampleDesc.Count, desc.SampleDesc.Quality);
    std::fputs(message, stderr);
  }
  D3D11_TEXTURE2D_DESC staging_desc = desc;
  staging_desc.Usage = D3D11_USAGE_STAGING;
  staging_desc.BindFlags = 0;
  staging_desc.CPUAccessFlags = D3D11_CPU_ACCESS_READ;
  staging_desc.MiscFlags = 0;
  ComPtr<ID3D11Texture2D> staging;
  hr = device->CreateTexture2D(&staging_desc, nullptr, staging.put());
  if (SUCCEEDED(hr)) context->CopyResource(staging.get(), texture.get());
  if (FAILED(hr)) {
    duplication->ReleaseFrame();
    return hr;
  }
  D3D11_MAPPED_SUBRESOURCE mapped{};
  hr = context->Map(staging.get(), 0, D3D11_MAP_READ, 0, &mapped);
  if (FAILED(hr)) {
    duplication->ReleaseFrame();
    return hr;
  }
  std::vector<uint8_t> raw(static_cast<size_t>(desc.Width) * desc.Height * 4);
  if (debug_enabled) {
    char message[160];
    std::snprintf(message, sizeof(message), "[daygo.capture] mapped row_pitch=%zu depth_pitch=%zu\n",
                  static_cast<size_t>(mapped.RowPitch), static_cast<size_t>(mapped.DepthPitch));
    std::fputs(message, stderr);
  }
  for (UINT y = 0; y < desc.Height; ++y) {
    std::copy_n(static_cast<const uint8_t*>(mapped.pData) + static_cast<size_t>(y) * mapped.RowPitch,
                static_cast<size_t>(desc.Width) * 4, raw.data() + static_cast<size_t>(y) * desc.Width * 4);
  }
  context->Unmap(staging.get(), 0);
  duplication->ReleaseFrame();
  const bool dxgi_all_zero = std::none_of(raw.begin(), raw.end(), [](uint8_t value) { return value != 0; });
  if (debug_enabled) {
    uint8_t min_value = 255;
    uint8_t max_value = 0;
    uint64_t nonzero = 0;
    for (uint8_t value : raw) {
      min_value = std::min(min_value, value);
      max_value = std::max(max_value, value);
      nonzero += value != 0;
    }
    char message[160];
    if (raw.size() >= 4) {
      std::snprintf(message, sizeof(message),
                    "[daygo.capture] pixels=%ux%u min=%u max=%u nonzero=%llu first=%u,%u,%u,%u\n",
                    desc.Width, desc.Height, min_value, max_value,
                    static_cast<unsigned long long>(nonzero), raw[0], raw[1], raw[2], raw[3]);
    } else {
      std::snprintf(message, sizeof(message), "[daygo.capture] pixels=%ux%u min=%u max=%u nonzero=%llu\n",
                    desc.Width, desc.Height, min_value, max_value, static_cast<unsigned long long>(nonzero));
    }
    OutputDebugStringA(message);
    std::fputs(message, stderr);
  }
  DXGI_OUTPUT_DESC output_desc{};
  output->GetDesc(&output_desc);
  UINT source_width = desc.Width;
  UINT source_height = desc.Height;
  DXGI_MODE_ROTATION source_rotation = output_desc.Rotation;
  if (dxgi_all_zero) {
    std::vector<uint8_t> gdi_pixels;
    UINT gdi_width = 0;
    UINT gdi_height = 0;
    if (!capture_gdi_primary(&gdi_pixels, &gdi_width, &gdi_height)) return E_FAIL;
    raw = std::move(gdi_pixels);
    source_width = gdi_width;
    source_height = gdi_height;
    source_rotation = DXGI_MODE_ROTATION_IDENTITY;
    debug_stage("dxgi.all_zero.gdi_fallback");
  }
  std::vector<uint8_t> oriented;
  UINT oriented_width = 0, oriented_height = 0;
  rotate_bgra(raw, source_width, source_height, source_rotation, &oriented, &oriented_width, &oriented_height);
  *pixels = scale_bgra(oriented, oriented_width, oriented_height, request.target_height, width);
  *height = request.target_height;
  FILETIME now{};
  GetSystemTimeAsFileTime(&now);
  ULARGE_INTEGER ticks{now.dwLowDateTime, now.dwHighDateTime};
  *captured_ns = static_cast<int64_t>((ticks.QuadPart - 116444736000000000ULL) * 100ULL);
  return S_OK;
}

}  // namespace

extern "C" {

DG_CAPTURE_API void DG_CAPTURE_CALL dg_capture_abi_version(uint32_t* major, uint32_t* minor) {
  if (major) *major = DG_CAPTURE_ABI_MAJOR;
  if (minor) *minor = DG_CAPTURE_ABI_MINOR;
}

DG_CAPTURE_API int32_t DG_CAPTURE_CALL dg_capture_once(uint32_t requested_abi_major,
                                                       const dg_capture_request_v1* request,
                                                       dg_capture_result_v1* result,
                                                       dg_capture_error_v1* error) {
  debug_stage("abi.enter");
  if (!request || !result) return DG_CAPTURE_E_INVALID_ARGUMENT;
  try {
  const uint32_t result_size = result->struct_size;
  result->image_format = 0;
  result->captured_at_unix_ns = 0;
  result->file_size = 0;
  result->width = 0;
  result->height = 0;
  result->struct_size = result_size;
  uint32_t error_size = 0;
  if (error) {
    error_size = error->struct_size;
    error->native_domain = DG_CAPTURE_NATIVE_NONE;
    error->native_code = 0;
    error->struct_size = error_size;
  }
  if (requested_abi_major != DG_CAPTURE_ABI_MAJOR || request->struct_size < sizeof(dg_capture_request_v1) ||
      result_size < sizeof(dg_capture_result_v1) || (error && error_size < sizeof(dg_capture_error_v1))) {
    return DG_CAPTURE_E_ABI_MISMATCH;
  }
  DecodedRequest decoded;
  if (!decode_request(*request, &decoded)) return DG_CAPTURE_E_INVALID_ARGUMENT;
  ComScope com;
  if (!com.usable()) return fail(DG_CAPTURE_E_UNSUPPORTED, com.result, error);
  if (!decoded.blocked_ids.empty()) {
    std::string frontmost;
    HRESULT privacy_hr = frontmost_application_id(&frontmost);
    if (FAILED(privacy_hr)) return fail(DG_CAPTURE_E_PRIVACY_UNSUPPORTED, privacy_hr, error);
    if (std::find(decoded.blocked_ids.begin(), decoded.blocked_ids.end(), frontmost) != decoded.blocked_ids.end()) return DG_CAPTURE_BLOCKED;
    // Desktop Duplication has no application-exclusion primitive. Do not
    // capture a potentially blocked window when the list is non-empty.
    return fail(DG_CAPTURE_E_PRIVACY_UNSUPPORTED, S_OK, error);
  }
  std::vector<uint8_t> pixels;
  UINT width = 0, height = 0;
  int64_t captured_ns = 0;
  HRESULT hr = capture_pixels(decoded, &pixels, &width, &height, &captured_ns);
  if (hr == DXGI_ERROR_WAIT_TIMEOUT) return fail(DG_CAPTURE_E_TIMEOUT, hr, error);
  if (hr == DXGI_ERROR_NOT_FOUND || hr == HRESULT_FROM_WIN32(ERROR_NOT_FOUND)) return fail(DG_CAPTURE_E_NO_DISPLAY, hr, error);
  if (FAILED(hr)) return fail(DG_CAPTURE_E_INTERNAL, hr, error);
  std::wstring temporary;
  hr = make_temp_path(decoded.output_path, &temporary);
  if (FAILED(hr)) return fail(DG_CAPTURE_E_IO, hr, error);
  bool temporary_exists = true;
  hr = encode_jpeg(temporary, pixels, width, height, decoded.jpeg_quality);
  if (SUCCEEDED(hr)) {
    HANDLE file = CreateFileW(temporary.c_str(), GENERIC_WRITE, FILE_SHARE_READ, nullptr, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, nullptr);
    if (file == INVALID_HANDLE_VALUE) hr = HRESULT_FROM_WIN32(GetLastError());
    else {
      if (!FlushFileBuffers(file)) hr = HRESULT_FROM_WIN32(GetLastError());
      CloseHandle(file);
    }
  }
  if (SUCCEEDED(hr) && !MoveFileExW(temporary.c_str(), decoded.output_path.c_str(), MOVEFILE_WRITE_THROUGH)) hr = HRESULT_FROM_WIN32(GetLastError());
  if (SUCCEEDED(hr)) temporary_exists = false;
  if (temporary_exists) DeleteFileW(temporary.c_str());
  if (FAILED(hr)) return fail(DG_CAPTURE_E_IO, hr, error);
  WIN32_FILE_ATTRIBUTE_DATA attributes{};
  if (!GetFileAttributesExW(decoded.output_path.c_str(), GetFileExInfoStandard, &attributes)) {
    DeleteFileW(decoded.output_path.c_str());
    return fail(DG_CAPTURE_E_IO, HRESULT_FROM_WIN32(GetLastError()), error);
  }
  ULARGE_INTEGER size{attributes.nFileSizeLow, attributes.nFileSizeHigh};
  result->image_format = DG_CAPTURE_IMAGE_JPEG;
  result->captured_at_unix_ns = captured_ns;
  result->file_size = size.QuadPart;
  result->width = width;
  result->height = height;
  debug_stage("abi.success");
  return DG_CAPTURE_OK;
  } catch (...) {
    return fail(DG_CAPTURE_E_INTERNAL, E_FAIL, error);
  }
}

}  // extern "C"
