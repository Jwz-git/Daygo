#ifndef NOMINMAX
#define NOMINMAX
#endif
#include "daygo_capture.h"

#include <windows.h>
#include <mfapi.h>
#include <mfidl.h>
#include <mfreadwrite.h>
#include <codecapi.h>
#include <wincodec.h>
#include <wrl/client.h>

#include <algorithm>
#include <chrono>
#include <cstdint>
#include <cstdio>
#include <filesystem>
#include <mutex>
#include <string>
#include <vector>

using Microsoft::WRL::ComPtr;
namespace fs = std::filesystem;

namespace {
constexpr LONGLONG kFrameDuration = 10'000'000;  // one second
constexpr uint32_t kMaxFrames = 600;
constexpr auto kMaxDuration = std::chrono::seconds(600);

std::mutex g_segment_mutex;
ComPtr<IMFSinkWriter> g_writer;
DWORD g_stream = 0;
uint32_t g_width = 0, g_height = 0, g_frames = 0;
std::chrono::steady_clock::time_point g_started;
std::wstring g_path;
std::string g_relative;

std::wstring widen(dg_capture_string_view_v1 view) {
  if (!view.data || view.len == 0 || view.len > 32768) return {};
  const int count = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS,
      reinterpret_cast<const char*>(view.data), static_cast<int>(view.len), nullptr, 0);
  if (count <= 0) return {};
  std::wstring out(count, L'\0');
  if (MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS,
      reinterpret_cast<const char*>(view.data), static_cast<int>(view.len), out.data(), count) != count) return {};
  return out;
}

int64_t unix_ns() {
  return std::chrono::duration_cast<std::chrono::nanoseconds>(
      std::chrono::system_clock::now().time_since_epoch()).count();
}

bool ensure_mf() {
  static const HRESULT result = [] {
    const HRESULT apartment = CoInitializeEx(nullptr, COINIT_MULTITHREADED);
    if (FAILED(apartment) && apartment != RPC_E_CHANGED_MODE) return apartment;
    return MFStartup(MF_VERSION, MFSTARTUP_FULL);
  }();
  return SUCCEEDED(result);
}

HRESULT decode_jpeg(const std::wstring& path, std::vector<uint8_t>* pixels,
                    uint32_t* width, uint32_t* height) {
  ComPtr<IWICImagingFactory> factory;
  HRESULT hr = CoCreateInstance(CLSID_WICImagingFactory, nullptr, CLSCTX_INPROC_SERVER,
                                IID_PPV_ARGS(&factory));
  ComPtr<IWICBitmapDecoder> decoder;
  if (SUCCEEDED(hr)) hr = factory->CreateDecoderFromFilename(path.c_str(), nullptr, GENERIC_READ,
      WICDecodeMetadataCacheOnDemand, &decoder);
  ComPtr<IWICBitmapFrameDecode> frame;
  if (SUCCEEDED(hr)) hr = decoder->GetFrame(0, &frame);
  ComPtr<IWICFormatConverter> converter;
  if (SUCCEEDED(hr)) hr = factory->CreateFormatConverter(&converter);
  if (SUCCEEDED(hr)) hr = converter->Initialize(frame.Get(), GUID_WICPixelFormat32bppBGRA,
      WICBitmapDitherTypeNone, nullptr, 0, WICBitmapPaletteTypeCustom);
  UINT w = 0, h = 0;
  if (SUCCEEDED(hr)) hr = converter->GetSize(&w, &h);
  if (FAILED(hr) || w == 0 || h == 0 || uint64_t(w) * h > (1ull << 30)) return FAILED(hr) ? hr : E_FAIL;
  pixels->resize(size_t(w) * h * 4);
  hr = converter->CopyPixels(nullptr, w * 4, static_cast<UINT>(pixels->size()), pixels->data());
  if (SUCCEEDED(hr)) { *width = w; *height = h; }
  return hr;
}

HRESULT finish_writer() {
  HRESULT hr = S_OK;
  if (g_writer) hr = g_writer->Finalize();
  g_writer.Reset(); g_frames = 0; g_width = 0; g_height = 0; g_path.clear(); g_relative.clear();
  return hr;
}

HRESULT start_writer(const std::wstring& root, uint32_t width, uint32_t height) {
  if (!ensure_mf()) return E_FAIL;
  std::error_code ec;
  fs::create_directories(fs::path(root) / L"segments", ec);
  if (ec) return HRESULT_FROM_WIN32(ec.value());
  const auto stamp = std::chrono::duration_cast<std::chrono::milliseconds>(
      std::chrono::system_clock::now().time_since_epoch()).count();
  wchar_t name[128]{};
  swprintf_s(name, L"daygo-%lld-%lu.mp4", static_cast<long long>(stamp), GetCurrentProcessId());
  g_path = (fs::path(root) / L"segments" / name).wstring();
  std::string utf8Name;
  const int bytes = WideCharToMultiByte(CP_UTF8, 0, name, -1, nullptr, 0, nullptr, nullptr);
  if (bytes <= 1) return E_FAIL;
  utf8Name.resize(bytes - 1);
  WideCharToMultiByte(CP_UTF8, 0, name, static_cast<int>(wcslen(name)), utf8Name.data(), bytes - 1, nullptr, nullptr);
  g_relative = "segments/" + utf8Name;

  HRESULT hr = MFCreateSinkWriterFromURL(g_path.c_str(), nullptr, nullptr, &g_writer);
  ComPtr<IMFMediaType> output;
  if (SUCCEEDED(hr)) hr = MFCreateMediaType(&output);
  if (SUCCEEDED(hr)) hr = output->SetGUID(MF_MT_MAJOR_TYPE, MFMediaType_Video);
  if (SUCCEEDED(hr)) hr = output->SetGUID(MF_MT_SUBTYPE, MFVideoFormat_HEVC);
  if (SUCCEEDED(hr)) hr = output->SetUINT32(MF_MT_AVG_BITRATE, std::max<uint32_t>(1'000'000, width * height * 2));
  if (SUCCEEDED(hr)) hr = MFSetAttributeSize(output.Get(), MF_MT_FRAME_SIZE, width, height);
  if (SUCCEEDED(hr)) hr = MFSetAttributeRatio(output.Get(), MF_MT_FRAME_RATE, 1, 1);
  if (SUCCEEDED(hr)) hr = MFSetAttributeRatio(output.Get(), MF_MT_PIXEL_ASPECT_RATIO, 1, 1);
  if (SUCCEEDED(hr)) hr = output->SetUINT32(MF_MT_INTERLACE_MODE, MFVideoInterlace_Progressive);
  if (SUCCEEDED(hr)) hr = g_writer->AddStream(output.Get(), &g_stream);
  ComPtr<IMFMediaType> input;
  if (SUCCEEDED(hr)) hr = MFCreateMediaType(&input);
  if (SUCCEEDED(hr)) hr = input->SetGUID(MF_MT_MAJOR_TYPE, MFMediaType_Video);
  if (SUCCEEDED(hr)) hr = input->SetGUID(MF_MT_SUBTYPE, MFVideoFormat_RGB32);
  if (SUCCEEDED(hr)) hr = MFSetAttributeSize(input.Get(), MF_MT_FRAME_SIZE, width, height);
  if (SUCCEEDED(hr)) hr = MFSetAttributeRatio(input.Get(), MF_MT_FRAME_RATE, 1, 1);
  if (SUCCEEDED(hr)) hr = MFSetAttributeRatio(input.Get(), MF_MT_PIXEL_ASPECT_RATIO, 1, 1);
  if (SUCCEEDED(hr)) hr = input->SetUINT32(MF_MT_INTERLACE_MODE, MFVideoInterlace_Progressive);
  if (SUCCEEDED(hr)) hr = g_writer->SetInputMediaType(g_stream, input.Get(), nullptr);
  if (SUCCEEDED(hr)) {
    ComPtr<ICodecAPI> codec;
    if (SUCCEEDED(g_writer->GetServiceForStream(g_stream, GUID_NULL, IID_PPV_ARGS(&codec)))) {
      VARIANT value{};
      value.vt = VT_UI4;
      value.ulVal = 30;
      codec->SetValue(&CODECAPI_AVEncMPVGOPSize, &value);
      value.ulVal = 55;
      codec->SetValue(&CODECAPI_AVEncCommonQuality, &value);
    }
  }
  if (SUCCEEDED(hr)) hr = g_writer->BeginWriting();
  if (FAILED(hr)) { finish_writer(); DeleteFileW(g_path.c_str()); return hr; }
  g_width = width; g_height = height; g_frames = 0; g_started = std::chrono::steady_clock::now();
  return S_OK;
}

HRESULT append_pixels(const std::wstring& root, const std::vector<uint8_t>& pixels,
                      uint32_t width, uint32_t height, uint32_t* index) {
  if (g_writer && (g_width != width || g_height != height || g_frames >= kMaxFrames ||
      std::chrono::steady_clock::now() - g_started >= kMaxDuration)) {
    const HRESULT hr = finish_writer(); if (FAILED(hr)) return hr;
  }
  if (!g_writer) { const HRESULT hr = start_writer(root, width, height); if (FAILED(hr)) return hr; }
  ComPtr<IMFMediaBuffer> buffer;
  HRESULT hr = MFCreateMemoryBuffer(static_cast<DWORD>(pixels.size()), &buffer);
  BYTE* data = nullptr;
  if (SUCCEEDED(hr)) hr = buffer->Lock(&data, nullptr, nullptr);
  if (SUCCEEDED(hr)) { memcpy(data, pixels.data(), pixels.size()); buffer->Unlock(); hr = buffer->SetCurrentLength(static_cast<DWORD>(pixels.size())); }
  ComPtr<IMFSample> sample;
  if (SUCCEEDED(hr)) hr = MFCreateSample(&sample);
  if (SUCCEEDED(hr)) hr = sample->AddBuffer(buffer.Get());
  if (SUCCEEDED(hr)) hr = sample->SetSampleTime(static_cast<LONGLONG>(g_frames) * kFrameDuration);
  if (SUCCEEDED(hr)) hr = sample->SetSampleDuration(kFrameDuration);
  if (SUCCEEDED(hr)) hr = g_writer->WriteSample(g_stream, sample.Get());
  if (SUCCEEDED(hr)) { *index = g_frames; ++g_frames; }
  return hr;
}

uint64_t file_size(const std::wstring& path) {
  WIN32_FILE_ATTRIBUTE_DATA data{};
  if (!GetFileAttributesExW(path.c_str(), GetFileExInfoStandard, &data)) return 0;
  return (uint64_t(data.nFileSizeHigh) << 32) | data.nFileSizeLow;
}

bool resolve_media_path(dg_capture_string_view_v1 root_view, dg_capture_string_view_v1 rel_view,
                        std::wstring* path) {
  const std::wstring root = widen(root_view), rel = widen(rel_view);
  if (root.empty() || rel.empty() || !fs::path(root).is_absolute() || fs::path(rel).is_absolute()) return false;
  const fs::path normalized = fs::path(rel).lexically_normal();
  if (normalized.empty() || *normalized.begin() == L"..") return false;
  *path = (fs::path(root) / normalized).wstring();
  return true;
}

HRESULT read_video_frame(const std::wstring& path, uint32_t wanted, std::vector<uint8_t>* pixels,
                         uint32_t* width, uint32_t* height, uint32_t* total) {
  if (!ensure_mf()) return E_FAIL;
  ComPtr<IMFSourceReader> reader;
  HRESULT hr = MFCreateSourceReaderFromURL(path.c_str(), nullptr, &reader);
  ComPtr<IMFMediaType> type;
  if (SUCCEEDED(hr)) hr = MFCreateMediaType(&type);
  if (SUCCEEDED(hr)) hr = type->SetGUID(MF_MT_MAJOR_TYPE, MFMediaType_Video);
  if (SUCCEEDED(hr)) hr = type->SetGUID(MF_MT_SUBTYPE, MFVideoFormat_RGB32);
  if (SUCCEEDED(hr)) hr = reader->SetCurrentMediaType(MF_SOURCE_READER_FIRST_VIDEO_STREAM, nullptr, type.Get());
  ComPtr<IMFMediaType> current;
  if (SUCCEEDED(hr)) hr = reader->GetCurrentMediaType(MF_SOURCE_READER_FIRST_VIDEO_STREAM, &current);
  UINT32 w = 0, h = 0;
  if (SUCCEEDED(hr)) hr = MFGetAttributeSize(current.Get(), MF_MT_FRAME_SIZE, &w, &h);
  if (FAILED(hr)) return hr;
  uint32_t count = 0;
  for (;;) {
    DWORD flags = 0;
    ComPtr<IMFSample> sample;
    hr = reader->ReadSample(MF_SOURCE_READER_FIRST_VIDEO_STREAM, 0, nullptr, &flags, nullptr, &sample);
    if (FAILED(hr)) return hr;
    if (flags & MF_SOURCE_READERF_ENDOFSTREAM) break;
    if (!sample) continue;
    if (count == wanted && pixels) {
      ComPtr<IMFMediaBuffer> buffer;
      hr = sample->ConvertToContiguousBuffer(&buffer);
      BYTE* data = nullptr; DWORD length = 0;
      if (SUCCEEDED(hr)) hr = buffer->Lock(&data, nullptr, &length);
      const size_t needed = size_t(w) * h * 4;
      if (SUCCEEDED(hr) && length >= needed) pixels->assign(data, data + needed);
      if (data) buffer->Unlock();
      if (FAILED(hr) || length < needed) return FAILED(hr) ? hr : E_FAIL;
    }
    ++count;
  }
  if (total) *total = count;
  if (pixels && wanted >= count) return MF_E_INVALIDINDEX;
  *width = w; *height = h;
  return S_OK;
}

HRESULT encode_jpeg_memory(const std::vector<uint8_t>& pixels, uint32_t width, uint32_t height,
                           uint32_t max_pixels, uint8_t** output, uint64_t* output_size) {
  ComPtr<IWICImagingFactory> factory;
  HRESULT hr = CoCreateInstance(CLSID_WICImagingFactory, nullptr, CLSCTX_INPROC_SERVER, IID_PPV_ARGS(&factory));
  ComPtr<IWICBitmap> bitmap;
  if (SUCCEEDED(hr)) hr = factory->CreateBitmapFromMemory(width, height, GUID_WICPixelFormat32bppBGRA,
      width * 4, static_cast<UINT>(pixels.size()), const_cast<BYTE*>(pixels.data()), &bitmap);
  ComPtr<IWICBitmapSource> source;
  if (SUCCEEDED(hr)) hr = bitmap.As(&source);
  ComPtr<IWICBitmapScaler> scaler;
  if (SUCCEEDED(hr) && max_pixels > 0 && std::max(width, height) > max_pixels) {
    const double scale = double(max_pixels) / std::max(width, height);
    const UINT scaled_w = std::max(1u, UINT(width * scale));
    const UINT scaled_h = std::max(1u, UINT(height * scale));
    hr = factory->CreateBitmapScaler(&scaler);
    if (SUCCEEDED(hr)) hr = scaler->Initialize(bitmap.Get(), scaled_w, scaled_h, WICBitmapInterpolationModeFant);
    if (SUCCEEDED(hr)) hr = scaler.As(&source);
  }
  ComPtr<IStream> stream;
  if (SUCCEEDED(hr)) hr = CreateStreamOnHGlobal(nullptr, TRUE, &stream);
  ComPtr<IWICBitmapEncoder> encoder;
  if (SUCCEEDED(hr)) hr = factory->CreateEncoder(GUID_ContainerFormatJpeg, nullptr, &encoder);
  if (SUCCEEDED(hr)) hr = encoder->Initialize(stream.Get(), WICBitmapEncoderNoCache);
  ComPtr<IWICBitmapFrameEncode> frame;
  if (SUCCEEDED(hr)) hr = encoder->CreateNewFrame(&frame, nullptr);
  if (SUCCEEDED(hr)) hr = frame->Initialize(nullptr);
  UINT out_w = width, out_h = height;
  if (scaler) scaler->GetSize(&out_w, &out_h);
  if (SUCCEEDED(hr)) hr = frame->SetSize(out_w, out_h);
  WICPixelFormatGUID format = GUID_WICPixelFormat32bppBGRA;
  if (SUCCEEDED(hr)) hr = frame->SetPixelFormat(&format);
  if (SUCCEEDED(hr)) hr = frame->WriteSource(source.Get(), nullptr);
  if (SUCCEEDED(hr)) hr = frame->Commit();
  if (SUCCEEDED(hr)) hr = encoder->Commit();
  HGLOBAL global = nullptr;
  if (SUCCEEDED(hr)) hr = GetHGlobalFromStream(stream.Get(), &global);
  const SIZE_T size = global ? GlobalSize(global) : 0;
  void* bytes = global ? GlobalLock(global) : nullptr;
  if (FAILED(hr) || !bytes || size == 0) return FAILED(hr) ? hr : E_FAIL;
  *output = static_cast<uint8_t*>(malloc(size));
  if (!*output) { GlobalUnlock(global); return E_OUTOFMEMORY; }
  memcpy(*output, bytes, size); GlobalUnlock(global); *output_size = size;
  return S_OK;
}
}  // namespace

extern "C" int32_t DG_CAPTURE_CALL dg_frame_append(uint32_t major,
    const dg_frame_append_request_v1* request, dg_frame_append_result_v1* result,
    dg_capture_error_v1* error) {
  if (major != DG_CAPTURE_ABI_MAJOR || !request || !result ||
      request->struct_size != sizeof(*request) || result->struct_size != sizeof(*result)) return DG_CAPTURE_E_INVALID_ARGUMENT;
  const std::wstring root = widen(request->recordings_dir);
  if (root.empty() || !fs::path(root).is_absolute()) return DG_CAPTURE_E_INVALID_ARGUMENT;
  std::lock_guard lock(g_segment_mutex);
  std::vector<uint8_t> pixels;
  uint32_t width = 0, height = 0;
  int32_t outcome = DG_CAPTURE_OK;
  if (request->synthetic_width && request->synthetic_height) {
    width = request->synthetic_width; height = request->synthetic_height;
    pixels.assign(size_t(width) * height * 4, 0x40);
  } else {
    const fs::path temporary = fs::path(root) / (L".daygo-capture-" + std::to_wstring(GetCurrentProcessId()) + L"-" + std::to_wstring(GetTickCount64()) + L".jpg");
    const std::string utf8 = temporary.u8string();
    dg_capture_request_v1 capture{}; capture.struct_size = sizeof(capture); capture.flags = request->flags;
    capture.image_format = DG_CAPTURE_IMAGE_JPEG; capture.target_height = request->target_height;
    capture.jpeg_quality = 90; capture.timeout_ms = request->timeout_ms;
    capture.blocked_application_id_count = request->blocked_application_id_count;
    capture.blocked_application_ids = request->blocked_application_ids;
    capture.output_path = {reinterpret_cast<const uint8_t*>(utf8.data()), utf8.size()};
    dg_capture_result_v1 captured{}; captured.struct_size = sizeof(captured);
    const int32_t status = dg_capture_once(major, &capture, &captured, error);
    if (status == DG_CAPTURE_BLOCKED) {
      outcome = status; height = request->target_height; width = std::max<uint32_t>(1, height * 16 / 9);
      pixels.assign(size_t(width) * height * 4, 0x30);
    } else if (status != DG_CAPTURE_OK) { return status; }
    else {
      const HRESULT hr = decode_jpeg(temporary.wstring(), &pixels, &width, &height);
      DeleteFileW(temporary.c_str());
      if (FAILED(hr)) return DG_CAPTURE_E_IO;
    }
  }
  uint32_t index = 0;
  const HRESULT hr = append_pixels(root, pixels, width, height, &index);
  if (FAILED(hr)) { if (error) { error->native_domain=DG_CAPTURE_NATIVE_WINDOWS; error->native_code=hr; } return DG_CAPTURE_E_IO; }
  result->outcome = outcome; result->frame_index = index; result->width = width; result->height = height;
  result->captured_at_unix_ns = unix_ns(); result->file_size = file_size(g_path);
  if (g_relative.size() >= sizeof(result->segment_rel_path)) return DG_CAPTURE_E_INTERNAL;
  memcpy(result->segment_rel_path, g_relative.c_str(), g_relative.size() + 1);
  return outcome;
}

extern "C" int32_t DG_CAPTURE_CALL dg_segment_close_active() {
  std::lock_guard lock(g_segment_mutex);
  return SUCCEEDED(finish_writer()) ? DG_CAPTURE_OK : DG_CAPTURE_E_IO;
}

extern "C" int32_t DG_CAPTURE_CALL dg_frame_decode(dg_capture_string_view_v1 root,
    dg_capture_string_view_v1 rel, uint32_t index, uint32_t max_pixels,
    uint8_t** output, uint64_t* output_size) {
  if (!output || !output_size) return DG_CAPTURE_E_INVALID_ARGUMENT;
  *output = nullptr; *output_size = 0;
  std::wstring path;
  if (!resolve_media_path(root, rel, &path)) return DG_CAPTURE_E_INVALID_ARGUMENT;
  const std::wstring extension = fs::path(path).extension().wstring();
  if (_wcsicmp(extension.c_str(), L".jpg") == 0 || _wcsicmp(extension.c_str(), L".jpeg") == 0) {
    HANDLE file = CreateFileW(path.c_str(), GENERIC_READ, FILE_SHARE_READ, nullptr, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, nullptr);
    if (file == INVALID_HANDLE_VALUE || index != 0) { if (file != INVALID_HANDLE_VALUE) CloseHandle(file); return DG_CAPTURE_E_IO; }
    LARGE_INTEGER size{};
    if (!GetFileSizeEx(file, &size) || size.QuadPart <= 0 || size.QuadPart > 64ll * 1024 * 1024) { CloseHandle(file); return DG_CAPTURE_E_IO; }
    *output = static_cast<uint8_t*>(malloc(static_cast<size_t>(size.QuadPart)));
    DWORD read = 0;
    const BOOL ok = *output && ReadFile(file, *output, static_cast<DWORD>(size.QuadPart), &read, nullptr);
    CloseHandle(file);
    if (!ok || read != size.QuadPart) { free(*output); *output=nullptr; return DG_CAPTURE_E_IO; }
    *output_size = read; return DG_CAPTURE_OK;
  }
  std::vector<uint8_t> pixels; uint32_t width = 0, height = 0;
  const HRESULT hr = read_video_frame(path, index, &pixels, &width, &height, nullptr);
  if (FAILED(hr) || FAILED(encode_jpeg_memory(pixels, width, height, max_pixels, output, output_size))) return DG_CAPTURE_E_IO;
  return DG_CAPTURE_OK;
}
extern "C" void DG_CAPTURE_CALL dg_frame_free(uint8_t* data) { free(data); }
extern "C" int32_t DG_CAPTURE_CALL dg_segment_probe(dg_capture_string_view_v1 root,
    dg_capture_string_view_v1 rel, dg_segment_info_v1* info) {
  if (!info || info->struct_size != sizeof(*info)) return DG_CAPTURE_E_INVALID_ARGUMENT;
  std::wstring path;
  if (!resolve_media_path(root, rel, &path)) return DG_CAPTURE_E_INVALID_ARGUMENT;
  const std::wstring extension = fs::path(path).extension().wstring();
  uint32_t width = 0, height = 0, count = 0;
  HRESULT hr;
  if (_wcsicmp(extension.c_str(), L".jpg") == 0 || _wcsicmp(extension.c_str(), L".jpeg") == 0) {
    std::vector<uint8_t> pixels; hr = decode_jpeg(path, &pixels, &width, &height); count = SUCCEEDED(hr) ? 1 : 0;
  } else {
    hr = read_video_frame(path, UINT32_MAX, nullptr, &width, &height, &count);
  }
  info->readable = SUCCEEDED(hr) ? 1 : 0; info->frame_count = count; info->width = width; info->height = height;
  return SUCCEEDED(hr) ? DG_CAPTURE_OK : DG_CAPTURE_E_IO;
}
