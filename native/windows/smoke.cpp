#define DAYGO_CAPTURE_STATIC 1
#include "../include/daygo_capture.h"

#include <windows.h>
#include <wincodec.h>

#include <cstdio>
#include <string>
#include <vector>

static std::string utf8(const std::wstring& value) {
  int bytes = WideCharToMultiByte(CP_UTF8, 0, value.data(), static_cast<int>(value.size()), nullptr, 0, nullptr, nullptr);
  std::string result(static_cast<size_t>(bytes), '\0');
  WideCharToMultiByte(CP_UTF8, 0, value.data(), static_cast<int>(value.size()), result.data(), bytes, nullptr, nullptr);
  return result;
}

static bool jpeg_has_nonblack_pixel(const std::wstring& path) {
  const HRESULT com_result = CoInitializeEx(nullptr, COINIT_MULTITHREADED);
  IWICImagingFactory* factory = nullptr;
  IWICBitmapDecoder* decoder = nullptr;
  IWICBitmapFrameDecode* frame = nullptr;
  IWICFormatConverter* converter = nullptr;
  bool nonblack = false;
  HRESULT hr = CoCreateInstance(CLSID_WICImagingFactory, nullptr, CLSCTX_INPROC_SERVER,
                                IID_PPV_ARGS(&factory));
  if (SUCCEEDED(hr)) hr = factory->CreateDecoderFromFilename(path.c_str(), nullptr, GENERIC_READ,
                                                              WICDecodeMetadataCacheOnDemand, &decoder);
  if (SUCCEEDED(hr)) hr = decoder->GetFrame(0, &frame);
  if (SUCCEEDED(hr)) hr = factory->CreateFormatConverter(&converter);
  if (SUCCEEDED(hr)) hr = converter->Initialize(frame, GUID_WICPixelFormat32bppBGRA,
                                                 WICBitmapDitherTypeNone, nullptr, 0.0,
                                                 WICBitmapPaletteTypeCustom);
  UINT width = 0;
  UINT height = 0;
  if (SUCCEEDED(hr)) hr = converter->GetSize(&width, &height);
  std::vector<uint8_t> pixels;
  if (SUCCEEDED(hr) && width != 0 && height != 0 && width <= 16384 && height <= 16384) {
    const UINT stride = width * 4;
    pixels.resize(static_cast<size_t>(stride) * height);
    hr = converter->CopyPixels(nullptr, stride, static_cast<UINT>(pixels.size()), pixels.data());
  }
  if (SUCCEEDED(hr)) {
    for (size_t offset = 0; offset + 3 < pixels.size(); offset += 4) {
      if (pixels[offset] != 0 || pixels[offset + 1] != 0 || pixels[offset + 2] != 0) {
        nonblack = true;
        break;
      }
    }
  }
  if (converter) converter->Release();
  if (frame) frame->Release();
  if (decoder) decoder->Release();
  if (factory) factory->Release();
  if (SUCCEEDED(com_result)) CoUninitialize();
  return nonblack;
}

int main() {
  uint32_t major = 0, minor = 0;
  dg_capture_abi_version(&major, &minor);
  if (major != DG_CAPTURE_ABI_MAJOR) {
    std::fprintf(stderr, "ABI major = %u, want %u\n", major, DG_CAPTURE_ABI_MAJOR);
    return 2;
  }

  wchar_t output_buffer[MAX_PATH] = {};
  DWORD output_length = GetEnvironmentVariableW(L"DAYGO_SMOKE_OUTPUT", output_buffer, MAX_PATH);
  std::wstring output_wide;
  if (output_length != 0 && output_length < MAX_PATH) {
    output_wide.assign(output_buffer, output_length);
  } else {
    output_wide = L"temp\\windows-capture-smoke.jpg";
    wchar_t absolute[MAX_PATH] = {};
    DWORD length = GetFullPathNameW(output_wide.c_str(), MAX_PATH, absolute, nullptr);
    if (length == 0 || length >= MAX_PATH) {
      std::fprintf(stderr, "GetFullPathNameW failed: %lu\n", GetLastError());
      return 2;
    }
    output_wide.assign(absolute, length);
  }
  const size_t slash = output_wide.find_last_of(L"\\/");
  if (slash != std::wstring::npos) {
    std::wstring directory = output_wide.substr(0, slash);
    if (!CreateDirectoryW(directory.c_str(), nullptr) && GetLastError() != ERROR_ALREADY_EXISTS) {
      std::fprintf(stderr, "CreateDirectoryW failed: %lu\n", GetLastError());
      return 2;
    }
  }
  DeleteFileW(output_wide.c_str());
  const std::string output = utf8(output_wide);
  dg_capture_request_v1 request{};
  request.struct_size = sizeof(request);
  request.image_format = DG_CAPTURE_IMAGE_JPEG;
  request.target_height = 720;
  request.jpeg_quality = 85;
  request.timeout_ms = 10000;
  request.output_path = {reinterpret_cast<const uint8_t*>(output.data()), output.size()};
  dg_capture_result_v1 result{sizeof(result)};
  dg_capture_error_v1 error{sizeof(error)};
  const int32_t status = dg_capture_once(DG_CAPTURE_ABI_MAJOR, &request, &result, &error);
  if (status != DG_CAPTURE_OK) {
    std::fprintf(stderr, "capture failed: status=%ld domain=%u native=%lld\n", static_cast<long>(status), error.native_domain, static_cast<long long>(error.native_code));
    return 1;
  }
  WIN32_FILE_ATTRIBUTE_DATA attributes{};
  if (!GetFileAttributesExW(output_wide.c_str(), GetFileExInfoStandard, &attributes)) {
    std::fprintf(stderr, "capture reported success but output is missing\n");
    return 1;
  }
  ULARGE_INTEGER size{attributes.nFileSizeLow, attributes.nFileSizeHigh};
  if (result.file_size != size.QuadPart || result.width == 0 || result.height != request.target_height) {
    std::fprintf(stderr, "invalid result: %ux%u, %llu bytes (disk %llu)\n", result.width, result.height,
                 static_cast<unsigned long long>(result.file_size), static_cast<unsigned long long>(size.QuadPart));
    return 1;
  }
  if (!jpeg_has_nonblack_pixel(output_wide)) {
    std::fprintf(stderr, "capture output contains no non-black pixels\n");
    return 1;
  }
  std::printf("capture ok: %ux%u, %llu bytes\n", result.width, result.height, static_cast<unsigned long long>(result.file_size));
  std::printf("capture image: %s\n", output.c_str());
  const std::wstring privacy_output_wide = output_wide.substr(0, output_wide.find_last_of(L'.')) + L"-privacy.jpg";
  DeleteFileW(privacy_output_wide.c_str());
  const std::string privacy_output = utf8(privacy_output_wide);
  request.output_path = {reinterpret_cast<const uint8_t*>(privacy_output.data()), privacy_output.size()};
  const char blocked_id[] = "com.example.blocked";
  dg_capture_string_view_v1 blocked_view{reinterpret_cast<const uint8_t*>(blocked_id), sizeof(blocked_id) - 1};
  request.blocked_application_id_count = 1;
  request.blocked_application_ids = &blocked_view;
  dg_capture_result_v1 blocked_result{sizeof(blocked_result)};
  dg_capture_error_v1 blocked_error{sizeof(blocked_error)};
  const int32_t blocked_status = dg_capture_once(DG_CAPTURE_ABI_MAJOR, &request, &blocked_result, &blocked_error);
  if (blocked_status != DG_CAPTURE_E_PRIVACY_UNSUPPORTED || GetFileAttributesW(privacy_output_wide.c_str()) != INVALID_FILE_ATTRIBUTES) {
    std::fprintf(stderr, "privacy guard expected privacy_unsupported, got status=%ld\n", static_cast<long>(blocked_status));
    return 1;
  }
  std::printf("privacy guard ok: privacy_unsupported\n");
  return 0;
}
