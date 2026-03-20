/**
 * @file camera_service.cpp
 * @brief カメラ取得サービス実装（P11-04）
 */
#include "camera_service.h"

#include <esp_camera.h>
#include <Wire.h>

#ifndef FW_CAMERA_ENABLED
#define FW_CAMERA_ENABLED 1
#endif

namespace {

// CoreS3 (GC0308) camera pin map.
constexpr int kCamPinPwdn  = -1;
constexpr int kCamPinReset = -1;
constexpr int kCamPinXclk  = 21;
constexpr int kCamPinSiod  = 12;
constexpr int kCamPinSioc  = 11;
constexpr int kCamPinD7    = 47;
constexpr int kCamPinD6    = 48;
constexpr int kCamPinD5    = 16;
constexpr int kCamPinD4    = 15;
constexpr int kCamPinD3    = 42;
constexpr int kCamPinD2    = 41;
constexpr int kCamPinD1    = 40;
constexpr int kCamPinD0    = 39;
constexpr int kCamPinVsync = 46;
constexpr int kCamPinHref  = 38;
constexpr int kCamPinPclk  = 45;

framesize_t toFrameSize(const String& resolution) {
  if (resolution.length() == 0) {
    return FRAMESIZE_QVGA;
  }

  String normalized = resolution;
  normalized.trim();
  normalized.toUpperCase();

  if (normalized == "QQVGA" || normalized == "160X120") return FRAMESIZE_QQVGA;
  if (normalized == "HQVGA" || normalized == "240X176") return FRAMESIZE_HQVGA;
  if (normalized == "QVGA" || normalized == "320X240") return FRAMESIZE_QVGA;
  if (normalized == "CIF" || normalized == "400X296") return FRAMESIZE_CIF;
  if (normalized == "VGA" || normalized == "640X480") return FRAMESIZE_VGA;
  if (normalized == "SVGA" || normalized == "800X600") return FRAMESIZE_SVGA;

  return FRAMESIZE_INVALID;
}

void fillBaseCameraConfig(camera_config_t* config) {
  *config = {};
  config->ledc_channel = LEDC_CHANNEL_0;
  config->ledc_timer = LEDC_TIMER_0;
  config->pin_d0 = kCamPinD0;
  config->pin_d1 = kCamPinD1;
  config->pin_d2 = kCamPinD2;
  config->pin_d3 = kCamPinD3;
  config->pin_d4 = kCamPinD4;
  config->pin_d5 = kCamPinD5;
  config->pin_d6 = kCamPinD6;
  config->pin_d7 = kCamPinD7;
  config->pin_xclk = kCamPinXclk;
  config->pin_pclk = kCamPinPclk;
  config->pin_vsync = kCamPinVsync;
  config->pin_href = kCamPinHref;
  config->pin_pwdn = kCamPinPwdn;
  config->pin_reset = kCamPinReset;
  config->xclk_freq_hz = 20000000;
  config->pixel_format = PIXFORMAT_JPEG;
  config->frame_size = FRAMESIZE_QVGA;
  config->jpeg_quality = 12;
  config->fb_count = 2;
  config->grab_mode = CAMERA_GRAB_LATEST;

  if (psramFound()) {
    config->fb_location = CAMERA_FB_IN_PSRAM;
  } else {
    config->fb_location = CAMERA_FB_IN_DRAM;
    config->fb_count = 1;
  }
}

bool tryInitCameraWithConfig(camera_config_t* config, const char* profileName) {
  const esp_err_t err = esp_camera_init(config);
  if (err == ESP_OK) {
    Serial.printf("[Camera] init success profile=%s\n", profileName);
    return true;
  }

  Serial.printf("[Camera] init failed profile=%s err=0x%x\n",
    profileName, static_cast<unsigned>(err));
  return false;
}

bool probeI2CAddress(TwoWire& wire, uint8_t address) {
  wire.beginTransmission(address);
  const uint8_t err = wire.endTransmission();
  return err == 0;
}

void logI2CDiagnostic(TwoWire& wire) {
  struct DeviceProbe {
    uint8_t addr;
    const char* name;
  };

  const DeviceProbe probes[] = {
    {0x21, "GC0308(camera)"},
    {0x23, "LTR553(proximity)"},
    {0x34, "AXP2101(pmic)"},
    {0x38, "FT6336(touch)"},
    {0x40, "ES7210(mic codec)"},
    {0x51, "BM8563(rtc)"},
    {0x58, "AW9523(io expander)"},
    {0x69, "BMI270(imu)"},
  };

  Serial.println("[Camera][I2C] probe start");
  for (const auto& probe : probes) {
    const bool found = probeI2CAddress(wire, probe.addr);
    Serial.printf("[Camera][I2C] addr=0x%02X %-22s %s\n",
      probe.addr,
      probe.name,
      found ? "FOUND" : "not found");
  }
}

}  // namespace

namespace Vision {

bool CameraService::initCameraDriver() {
  camera_config_t config;

  // CoreS3 では M5.begin() の構成により I2C ドライバが未初期化な場合があるため、
  // カメラ初期化前に SCCB 用バスを明示初期化しておきます。
  if (!Wire.begin(kCamPinSiod, kCamPinSioc, 400000U)) {
    Serial.println("[Camera] WARN: Wire.begin failed for camera SCCB pins");
  } else {
    Serial.printf("[Camera] Wire.begin ok sda=%d scl=%d\n", kCamPinSiod, kCamPinSioc);
  }

  logI2CDiagnostic(Wire);

  if (!probeI2CAddress(Wire, 0x21)) {
    Serial.println("[Camera] GC0308(0x21) not found on I2C bus; camera ribbon/camera power/hardware likely issue");
    return false;
  }

  // 1) 専用 SCCB バスとして初期化（従来方式）
  fillBaseCameraConfig(&config);
  config.pin_sccb_sda = kCamPinSiod;
  config.pin_sccb_scl = kCamPinSioc;
  config.sccb_i2c_port = 0;
  if (tryInitCameraWithConfig(&config, "dedicated-sccb-port0")) {
    return true;
  }

  // 1.5) 専用 SCCB を I2C1 でも試行（I2C0 競合回避）
  fillBaseCameraConfig(&config);
  config.pin_sccb_sda = kCamPinSiod;
  config.pin_sccb_scl = kCamPinSioc;
  config.sccb_i2c_port = 1;
  if (tryInitCameraWithConfig(&config, "dedicated-sccb-port1")) {
    return true;
  }

  // 2) M5Unified が確保済みの I2C0 を共有して初期化
  fillBaseCameraConfig(&config);
  config.pin_sccb_sda = -1;
  config.pin_sccb_scl = -1;
  config.sccb_i2c_port = 0;
  if (tryInitCameraWithConfig(&config, "shared-i2c0")) {
    return true;
  }

  // 3) I2C1 共有で再試行（ボード差分吸収）
  fillBaseCameraConfig(&config);
  config.pin_sccb_sda = -1;
  config.pin_sccb_scl = -1;
  config.sccb_i2c_port = 1;
  if (tryInitCameraWithConfig(&config, "shared-i2c1")) {
    return true;
  }

  // 4) XCLK 無効モード（ボード実装差分吸収）
  fillBaseCameraConfig(&config);
  config.pin_xclk = -1;
  config.pin_sccb_sda = -1;
  config.pin_sccb_scl = -1;
  config.sccb_i2c_port = 0;
  if (tryInitCameraWithConfig(&config, "shared-i2c0-no-xclk")) {
    return true;
  }

  Serial.println("[Camera] all init profiles failed");
  return false;
}

void CameraService::begin() {
  _initialized = true;

#if !FW_CAMERA_ENABLED
  _enabled = false;
  _available = false;
  Serial.println("[Camera] CameraService disabled by config (FW_CAMERA_ENABLED=0)");
  return;
#else
  _enabled = true;
  _available = initCameraDriver();

  if (_available) {
    Serial.println("[Camera] CameraService initialized (capture backend ready)");
  } else {
    Serial.println("[Camera] CameraService initialized (camera unavailable)");
  }
#endif
}

CaptureResult CameraService::capture(const CaptureRequest& request) {
  CaptureResult result;
  result.captureId = request.requestId;

  if (!_initialized) {
    result.accepted = false;
    result.reason = "camera service not initialized";
    return result;
  }

  if (!_enabled) {
    result.accepted = false;
    result.reason = "camera disabled by config";
    return result;
  }

  if (!_available) {
    result.accepted = false;
    result.reason = "camera not available";
    return result;
  }

  sensor_t* sensor = esp_camera_sensor_get();
  if (sensor == nullptr) {
    _available = false;
    result.accepted = false;
    result.reason = "camera sensor unavailable";
    return result;
  }

  if (request.resolution.length() > 0) {
    const framesize_t targetSize = toFrameSize(request.resolution);
    if (targetSize == FRAMESIZE_INVALID) {
      result.accepted = false;
      result.reason = "unsupported resolution";
      return result;
    }
    sensor->set_framesize(sensor, targetSize);
  }

  if (request.quality >= 0) {
    const int quality = constrain(request.quality, 10, 63);
    sensor->set_quality(sensor, quality);
  }

  camera_fb_t* frame = esp_camera_fb_get();
  if (frame == nullptr) {
    result.accepted = false;
    result.reason = "capture failed";
    return result;
  }

  result.accepted = true;
  result.reason = "accepted";
  result.capturedAtMs = millis();
  result.imageBytes = frame->len;
  result.width = frame->width;
  result.height = frame->height;

  esp_camera_fb_return(frame);
  return result;
}

}  // namespace Vision
