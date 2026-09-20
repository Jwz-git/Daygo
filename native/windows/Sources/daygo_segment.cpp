#ifndef NOMINMAX
#define NOMINMAX
#endif
#include "daygo_capture.h"

#include <windows.h>
#include <mfapi.h>
#include <mfidl.h>
#include <mfreadwrite.h>
#include <codecapi.h>
#include <icodecapi.h>
#include <mferror.h>
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

// JPEG fallback quality. Only reached when no usable video encoder exists, so
// this trades size for the guarantee that the frame can be written at all.
constexpr uint32_t kFallbackJPEGQuality = 90;

// Segment codec, in preference order. HEVC is an optional Windows component (an
// OEM-supplied encoder or the Store "HEVC Video Extensions" package) and is
// missing on many machines; H.264 ships with Media Foundation on every non-N
// edition; WIC JPEG is always present. A resident recorder must not stop because
// the preferred encoder is absent, so the adapter resolves the best available
// codec once and degrades instead of failing every frame.
enum class SegmentCodec { kHevc = 0, kH264 = 1, kJpeg = 2 };

std::mutex g_segment_mutex;
ComPtr<IMFSinkWriter> g_writer;
DWORD g_stream = 0;
uint32_t g_width = 0, g_height = 0, g_frames = 0;
std::chrono::steady_clock::time_point g_started;
std::wstring g_path;
std::string g_relative;
SegmentCodec g_codec = SegmentCodec::kHevc;
bool g_codec_resolved = false;
uint64_t g_segment_sequence = 0;

bool segment_debug_enabled() {
  char enabled[2] = {};
  const DWORD count = GetEnvironmentVariableA("DAYGO_CAPTURE_DEBUG", enabled, sizeof(enabled));
  return count == 1 && enabled[0] == '1';
}

// Reports the codec decision without leaking paths or image content. `codec` is
// the SegmentCodec ordinal; `stage` names the transition.
void segment_debug_codec(const char* stage, SegmentCodec codec) {
  if (!segment_debug_enabled()) return;
  std::fprintf(stderr, "[daygo.segment] stage=%s codec=%d\n", stage, static_cast<int>(codec));
}

const GUID& codec_subtype(SegmentCodec codec) {
  return codec == SegmentCodec::kHevc ? MFVideoFormat_HEVC : MFVideoFormat_H264;
}

// COM must be initialized on the calling thread for WIC and Media Foundation.
// Go goroutines migrate between OS threads, so each ABI entry point establishes
// its own apartment for the duration of the call rather than assuming one was
// set up earlier on this thread.
struct ComScope {
  HRESULT result;
  ComScope() : result(CoInitializeEx(nullptr, COINIT_MULTITHREADED)) {}
  ~ComScope() {
    if (SUCCEEDED(result)) CoUninitialize();
  }
};

bool wide_to_utf8(const std::wstring& input, std::string* output) {
  const int bytes = WideCharToMultiByte(CP_UTF8, 0, input.c_str(), static_cast<int>(input.size()),
                                        nullptr, 0, nullptr, nullptr);
  if (bytes <= 0) return false;
  output->resize(static_cast<size_t>(bytes));
  return WideCharToMultiByte(CP_UTF8, 0, input.c_str(), static_cast<int>(input.size()),
                             output->data(), bytes, nullptr, nullptr) == bytes;
}

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

// segment_name produces a unique path for a new segment under `root`. The
// in-process sequence keeps segments built within the same millisecond distinct.
bool segment_name(const std::wstring& root, const wchar_t* extension, std::wstring* path,
                  std::string* relative) {
  std::error_code ec;
  const fs::path directory = fs::path(root) / L"segments";
  fs::create_directories(directory, ec);
  if (ec) return false;
  const auto stamp = std::chrono::duration_cast<std::chrono::milliseconds>(
      std::chrono::system_clock::now().time_since_epoch()).count();
  wchar_t name[160]{};
  swprintf_s(name, L"daygo-%lld-%lu-%llu.%s", static_cast<long long>(stamp),
             GetCurrentProcessId(), static_cast<unsigned long long>(++g_segment_sequence), extension);
  std::string utf8_name;
  if (!wide_to_utf8(name, &utf8_name)) return false;
  *path = (directory / name).wstring();
  *relative = "segments/" + utf8_name;
  return true;
}

// build_writer creates a sink writer for `codec` at `path` and starts it. Every
// failure path releases the writer and deletes the partial file, so the caller
// can try the next codec at the same path without leaving debris behind.
HRESULT build_writer(const std::wstring& path, uint32_t width, uint32_t height,
                     SegmentCodec codec, ComPtr<IMFSinkWriter>* writer, DWORD* stream) {
  HRESULT hr = MFCreateSinkWriterFromURL(path.c_str(), nullptr, nullptr, &(*writer));
  ComPtr<IMFMediaType> output;
  if (SUCCEEDED(hr)) hr = MFCreateMediaType(&output);
  if (SUCCEEDED(hr)) hr = output->SetGUID(MF_MT_MAJOR_TYPE, MFMediaType_Video);
  if (SUCCEEDED(hr)) hr = output->SetGUID(MF_MT_SUBTYPE, codec_subtype(codec));
  if (SUCCEEDED(hr)) hr = output->SetUINT32(MF_MT_AVG_BITRATE, std::max<uint32_t>(1'000'000, width * height * 2));
  if (SUCCEEDED(hr)) hr = MFSetAttributeSize(output.Get(), MF_MT_FRAME_SIZE, width, height);
  if (SUCCEEDED(hr)) hr = MFSetAttributeRatio(output.Get(), MF_MT_FRAME_RATE, 1, 1);
  if (SUCCEEDED(hr)) hr = MFSetAttributeRatio(output.Get(), MF_MT_PIXEL_ASPECT_RATIO, 1, 1);
  if (SUCCEEDED(hr)) hr = output->SetUINT32(MF_MT_INTERLACE_MODE, MFVideoInterlace_Progressive);
  if (SUCCEEDED(hr)) hr = (*writer)->AddStream(output.Get(), stream);
  ComPtr<IMFMediaType> input;
  if (SUCCEEDED(hr)) hr = MFCreateMediaType(&input);
  if (SUCCEEDED(hr)) hr = input->SetGUID(MF_MT_MAJOR_TYPE, MFMediaType_Video);
  if (SUCCEEDED(hr)) hr = input->SetGUID(MF_MT_SUBTYPE, MFVideoFormat_RGB32);
  if (SUCCEEDED(hr)) hr = MFSetAttributeSize(input.Get(), MF_MT_FRAME_SIZE, width, height);
  if (SUCCEEDED(hr)) hr = MFSetAttributeRatio(input.Get(), MF_MT_FRAME_RATE, 1, 1);
  if (SUCCEEDED(hr)) hr = MFSetAttributeRatio(input.Get(), MF_MT_PIXEL_ASPECT_RATIO, 1, 1);
  if (SUCCEEDED(hr)) hr = input->SetUINT32(MF_MT_INTERLACE_MODE, MFVideoInterlace_Progressive);
  // Declare the row order of the frame buffers append_pixels writes. The
  // capture path hands over top-down rows, and Media Foundation reads an
  // RGB32 buffer without this attribute as bottom-up — RGB images in system
  // memory usually are, and MFGetStrideForBitmapInfoHeader answers with a
  // negative stride for RGB32. Declaring the positive stride is what keeps the
  // stored picture the right way up: without it every segment is written
  // vertically mirrored, which no reader can undo because the flip is baked
  // into the coded frame. Both the HEVC and the H.264 encoder honor it.
  const uint32_t stride = width * 4;
  if (SUCCEEDED(hr)) hr = input->SetUINT32(MF_MT_DEFAULT_STRIDE, stride);
  if (SUCCEEDED(hr)) hr = (*writer)->SetInputMediaType(*stream, input.Get(), nullptr);
  if (SUCCEEDED(hr)) {
    ComPtr<ICodecAPI> encoder_options;
    if (SUCCEEDED((*writer)->GetServiceForStream(*stream, GUID{}, IID_PPV_ARGS(&encoder_options)))) {
      VARIANT value{};
      value.vt = VT_UI4;
      value.ulVal = 30;
      encoder_options->SetValue(&CODECAPI_AVEncMPVGOPSize, &value);
      value.ulVal = 55;
      encoder_options->SetValue(&CODECAPI_AVEncCommonQuality, &value);
    }
  }
  if (SUCCEEDED(hr)) hr = (*writer)->BeginWriting();
  if (FAILED(hr)) {
    writer->Reset();
    *stream = 0;
    DeleteFileW(path.c_str());
  }
  return hr;
}

// start_writer resolves the segment codec on first use and opens a segment.
// MF_E_TOPO_CODEC_NOT_FOUND means no video encoder works at all on this machine,
// which tells the caller to use the per-frame JPEG path instead.
HRESULT start_writer(const std::wstring& root, uint32_t width, uint32_t height) {
  if (!ensure_mf()) return E_FAIL;
  std::wstring path;
  std::string relative;
  if (!segment_name(root, L"mp4", &path, &relative)) return E_FAIL;

  // Once resolved, the working codec is tried first; a segment still walks the
  // remaining lower codecs so a one-off failure does not end the process.
  const int first = g_codec_resolved ? static_cast<int>(g_codec) : static_cast<int>(SegmentCodec::kHevc);
  for (int index = first; index <= static_cast<int>(SegmentCodec::kH264); ++index) {
    const SegmentCodec candidate = static_cast<SegmentCodec>(index);
    ComPtr<IMFSinkWriter> writer;
    DWORD stream = 0;
    if (SUCCEEDED(build_writer(path, width, height, candidate, &writer, &stream))) {
      g_codec = candidate;
      g_codec_resolved = true;
      g_writer = writer;
      g_stream = stream;
      g_path = path;
      g_relative = relative;
      g_width = width;
      g_height = height;
      g_frames = 0;
      g_started = std::chrono::steady_clock::now();
      segment_debug_codec("codec.resolved", candidate);
      return S_OK;
    }
  }

  g_codec = SegmentCodec::kJpeg;
  g_codec_resolved = true;
  segment_debug_codec("codec.resolved", SegmentCodec::kJpeg);
  return MF_E_TOPO_CODEC_NOT_FOUND;
}

// degrade_codec steps down the preference chain after the resolved codec failed
// to encode a real frame. Only the first frame of a segment may degrade: once
// frames are committed to a file, switching codecs would leave a segment that no
// single decoder can read.
void degrade_codec() {
  g_codec = g_codec == SegmentCodec::kHevc ? SegmentCodec::kH264 : SegmentCodec::kJpeg;
  g_codec_resolved = true;
}

// encode_jpeg_file writes tightly packed BGRA `pixels` to `path` as a JPEG. The
// image is encoded beside the target and published with a write-through rename,
// so an interrupted write cannot leave a truncated frame in the segment listing.
HRESULT encode_jpeg_file(const std::wstring& path, const std::vector<uint8_t>& pixels,
                         uint32_t width, uint32_t height) {
  const std::wstring temporary = path + L".partial";
  DeleteFileW(temporary.c_str());
  HRESULT hr = S_OK;
  {
    ComPtr<IWICImagingFactory> factory;
    hr = CoCreateInstance(CLSID_WICImagingFactory, nullptr, CLSCTX_INPROC_SERVER, IID_PPV_ARGS(&factory));
    ComPtr<IWICStream> stream;
    if (SUCCEEDED(hr)) hr = factory->CreateStream(&stream);
    if (SUCCEEDED(hr)) hr = stream->InitializeFromFilename(temporary.c_str(), GENERIC_WRITE);
    ComPtr<IWICBitmapEncoder> encoder;
    if (SUCCEEDED(hr)) hr = factory->CreateEncoder(GUID_ContainerFormatJpeg, nullptr, &encoder);
    if (SUCCEEDED(hr)) hr = encoder->Initialize(stream.Get(), WICBitmapEncoderNoCache);
    ComPtr<IWICBitmapFrameEncode> frame;
    ComPtr<IPropertyBag2> options;
    if (SUCCEEDED(hr)) hr = encoder->CreateNewFrame(&frame, &options);
    if (SUCCEEDED(hr) && options) {
      PROPBAG2 option{};
      option.pstrName = const_cast<LPOLESTR>(L"ImageQuality");
      VARIANT value;
      VariantInit(&value);
      value.vt = VT_R4;
      value.fltVal = static_cast<float>(kFallbackJPEGQuality) / 100.0f;
      options->Write(1, &option, &value);
      VariantClear(&value);
    }
    if (SUCCEEDED(hr)) hr = frame->Initialize(options.Get());
    if (SUCCEEDED(hr)) hr = frame->SetSize(width, height);
    WICPixelFormatGUID format = GUID_WICPixelFormat24bppBGR;
    if (SUCCEEDED(hr)) hr = frame->SetPixelFormat(&format);
    ComPtr<IWICBitmap> bitmap;
    if (SUCCEEDED(hr)) hr = factory->CreateBitmapFromMemory(width, height, GUID_WICPixelFormat32bppBGRA,
                                                            width * 4, static_cast<UINT>(pixels.size()),
                                                            const_cast<BYTE*>(pixels.data()), &bitmap);
    ComPtr<IWICFormatConverter> converter;
    if (SUCCEEDED(hr)) hr = factory->CreateFormatConverter(&converter);
    if (SUCCEEDED(hr)) hr = converter->Initialize(bitmap.Get(), GUID_WICPixelFormat24bppBGR,
                                                  WICBitmapDitherTypeNone, nullptr, 0.0,
                                                  WICBitmapPaletteTypeCustom);
    if (SUCCEEDED(hr)) hr = frame->WriteSource(converter.Get(), nullptr);
    if (SUCCEEDED(hr)) hr = frame->Commit();
    if (SUCCEEDED(hr)) hr = encoder->Commit();
  }
  if (SUCCEEDED(hr) && !MoveFileExW(temporary.c_str(), path.c_str(), MOVEFILE_WRITE_THROUGH)) {
    hr = HRESULT_FROM_WIN32(GetLastError());
  }
  if (FAILED(hr)) DeleteFileW(temporary.c_str());
  return hr;
}

// append_jpeg_frame writes one frame as its own single-frame segment, used only
// when no video encoder is usable. The (segment_path, frame_index) contract is
// preserved by giving every frame a distinct path and index 0: probe, decode,
// cleanup and amortization already handle the `.jpg` form.
HRESULT append_jpeg_frame(const std::wstring& root, const std::vector<uint8_t>& pixels,
                          uint32_t width, uint32_t height, uint32_t* index) {
  std::wstring path;
  std::string relative;
  if (!segment_name(root, L"jpg", &path, &relative)) return E_FAIL;
  const HRESULT hr = encode_jpeg_file(path, pixels, width, height);
  if (FAILED(hr)) {
    DeleteFileW(path.c_str());
    return hr;
  }
  g_path = path;
  g_relative = relative;
  g_width = width;
  g_height = height;
  g_frames = 0;
  *index = 0;
  return S_OK;
}

HRESULT append_pixels(const std::wstring& root, const std::vector<uint8_t>& pixels,
                      uint32_t width, uint32_t height, uint32_t* index) {
  if (g_writer && (g_width != width || g_height != height || g_frames >= kMaxFrames ||
      std::chrono::steady_clock::now() - g_started >= kMaxDuration)) {
    const HRESULT hr = finish_writer(); if (FAILED(hr)) return hr;
  }
  // The JPEG path owns no container writer, so the rollover above never applies
  // to it: every frame is already its own finalized segment.
  if (g_codec_resolved && g_codec == SegmentCodec::kJpeg) {
    return append_jpeg_frame(root, pixels, width, height, index);
  }
  if (!g_writer) {
    const HRESULT started = start_writer(root, width, height);
    if (started == MF_E_TOPO_CODEC_NOT_FOUND) return append_jpeg_frame(root, pixels, width, height, index);
    if (FAILED(started)) return started;
  }
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
  if (SUCCEEDED(hr)) { *index = g_frames; ++g_frames; return hr; }
  if (g_frames == 0) {
    // The writer accepted the stream but rejected the very first real frame, so
    // the resolved codec is not usable at this resolution after all. Drop the
    // empty segment and step down the chain: the recorder's next attempt lands
    // on the fallback codec instead of spending its failure budget here.
    const std::wstring empty_segment = g_path;
    finish_writer();
    DeleteFileW(empty_segment.c_str());
    degrade_codec();
    segment_debug_codec("codec.degraded", g_codec);
  }
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
  // The decoder for an HEVC segment outputs NV12. The source reader only
  // inserts the video processor that converts it to RGB32 when advanced video
  // processing is enabled; without the attribute SetCurrentMediaType below
  // fails with MF_E_INVALIDMEDIATYPE and the segment reads back as unplayable.
  ComPtr<IMFAttributes> attributes;
  HRESULT hr = MFCreateAttributes(&attributes, 1);
  if (SUCCEEDED(hr)) hr = attributes->SetUINT32(MF_SOURCE_READER_ENABLE_ADVANCED_VIDEO_PROCESSING, TRUE);
  ComPtr<IMFSourceReader> reader;
  if (SUCCEEDED(hr)) hr = MFCreateSourceReaderFromURL(path.c_str(), attributes.Get(), &reader);
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

// The image a synthetic append writes is the test's own, so it is shaped to
// reveal something a screenshot cannot: the top and bottom halves carry
// different values, which makes a vertical flip visible when the frame is read
// back. That is how the smoke test guards the row order the writer declares to
// Media Foundation. The two values only have to be far enough apart to survive
// the encode, so they are placeholders rather than content.
constexpr uint8_t kSyntheticTopHalf = 0xC0;
constexpr uint8_t kSyntheticBottomHalf = 0x20;

std::vector<uint8_t> synthetic_frame(uint32_t width, uint32_t height) {
  std::vector<uint8_t> pixels(size_t(width) * height * 4);
  for (uint32_t y = 0; y < height; ++y) {
    const uint8_t value = (y < height / 2) ? kSyntheticTopHalf : kSyntheticBottomHalf;
    for (uint32_t x = 0; x < width; ++x) {
      uint8_t* pixel = &pixels[(size_t(y) * width + x) * 4];
      pixel[0] = pixel[1] = pixel[2] = value;
      pixel[3] = 0xFF;
    }
  }
  return pixels;
}
}  // namespace

extern "C" int32_t DG_CAPTURE_CALL dg_frame_append(uint32_t major,
    const dg_frame_append_request_v1* request, dg_frame_append_result_v1* result,
    dg_capture_error_v1* error) {
  if (major != DG_CAPTURE_ABI_MAJOR || !request || !result ||
      request->struct_size != sizeof(*request) || result->struct_size != sizeof(*result)) return DG_CAPTURE_E_INVALID_ARGUMENT;
  const std::wstring root = widen(request->recordings_dir);
  if (root.empty() || !fs::path(root).is_absolute()) return DG_CAPTURE_E_INVALID_ARGUMENT;
  // Go goroutines migrate between OS threads, so this call cannot assume an
  // earlier call initialized COM here; WIC and Media Foundation both need it.
  ComScope com;
  std::lock_guard lock(g_segment_mutex);
  std::vector<uint8_t> pixels;
  uint32_t width = 0, height = 0;
  int32_t outcome = DG_CAPTURE_OK;
  if (request->synthetic_width && request->synthetic_height) {
    width = request->synthetic_width; height = request->synthetic_height;
    pixels = synthetic_frame(width, height);
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
  ComScope com;
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
  ComScope com;
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
