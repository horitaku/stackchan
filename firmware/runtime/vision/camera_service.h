/**
 * @file camera_service.h
 * @brief カメラ取得サービス（P11-04）
 *
 * Session からカメラ依存を分離するためのサービス境界です。
 * CoreS3 では esp_camera を用いた静止画取得を行います。
 */
#pragma once

#include <Arduino.h>

namespace Vision {

/**
 * @brief 静止画取得要求です。
 */
struct CaptureRequest {
  String requestId;
  String resolution;
  int quality{-1};
};

/**
 * @brief 静止画取得結果です。
 */
struct CaptureResult {
  bool accepted{false};
  String reason;
  String captureId;
  unsigned long capturedAtMs{0};
  size_t imageBytes{0};
  int width{0};
  int height{0};
};

/**
 * @brief カメラサービスです。
 */
class CameraService {
 public:
  CameraService() = default;

  /**
   * @brief サービスを初期化します。
   */
  void begin();

  /**
    * @brief カメラ機能が設定上有効かを返します。
    */
    bool enabled() const { return _enabled; }

    /**
   * @brief カメラが利用可能かを返します。
   */
  bool available() const { return _available; }

  /**
   * @brief 静止画取得要求を処理します。
   */
  CaptureResult capture(const CaptureRequest& request);

 private:
  bool initCameraDriver();

  bool _initialized{false};
  bool _enabled{true};
  bool _available{false};
};

}  // namespace Vision
