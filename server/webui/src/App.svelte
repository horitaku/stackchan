<script>
  import { onMount } from "svelte";

  const thresholds = {
    totalLatencyMs: 3000,
    decodeErrorCount: 1,
    outputErrorCount: 1
  };

  let snapshot = {
    connection: {},
    playback: {},
    pipeline: {},
    avatar: {},
    hardware: {},
    updated_at: "-"
  };
  let settings = {
    playback_volume: 70,
    expression_preset: "neutral",
    lip_sync_sensitivity: 1,
    lip_sync_damping: 0.3,
    motion_enabled: true,
    updated_at: "-"
  };
  let settingsMessage = "";
  let llmSettings = {
    system_prompt: "Stack-chan です。話しかけてくれてありがとう。",
    updated_at: "-"
  };
  let llmSettingsMessage = "";
  let overviewMessage = "";
  let pipelineTestResult = "未実行";
  let voicevoxText = "こんにちは、Stackchan の Voicevox 単体テストです。";
  let voicevoxSpeaker = 1;
  let voicevoxResult = "未実行";
  let voicevoxAudioSrc = "";
  let stackchanText = "こんにちは、Stackchan 連携テストです。";
  let stackchanSpeaker = 1;
  let stackchanExpression = "happy";
  let stackchanMotion = "nod";
  let stackchanChunkVersion = "1.0";
  let stackchanResult = "未実行";
  let llmUIText = "こんにちは、自己紹介して";
  let llmUIPersona = "";
  let llmUIResult = "未実行";
  let llmStackchanText = "きょうの気分をひとことください";
  let llmStackchanPersona = "";
  let llmStackchanSpeaker = 1;
  let llmStackchanExpression = "happy";
  let llmStackchanMotion = "nod";
  let llmStackchanChunkVersion = "1.1";
  let llmStackchanResult = "未実行";
  let hardwareStatus = "未実行";
  let hardwareStatusUpdatedAt = "-";
  let speakerToneHz = 440;
  let speakerDurationMs = 800;
  let speakerVolume = 0.8;
  let speakerTestResult = "未実行";
  let micDurationMs = 1200;
  let micSampleRateHz = 16000;
  let micFrameDurationMs = 20;
  let micTestResult = "未実行";
  let touchStateLabel = "未取得";

  let servoAxis = "both";
  let servoXDeg = 0;
  let servoYDeg = 0;
  let servoSpeed = 1.0;
  let servoCalAxis = "x";
  let servoCenterOffsetDeg = 0;
  let servoMinDeg = -45;
  let servoMaxDeg = 45;
  let servoInvert = false;
  let servoSpeedLimitDegPerSec = 60;
  let servoSoftStart = true;
  let servoHomeDeg = 0;
  let servoResult = "未実行";

  let ledMode = "solid";
  let ledColor = "#00BFFF";
  let ledBrightness = 180;
  let ledBlinkIntervalMs = 500;
  let ledBreathePeriodMs = 2000;
  let ledResult = "未実行";

  let earsMode = "rainbow";
  let earsColor = "#FF69B4";
  let earsBrightness = 180;
  let earsBlinkIntervalMs = 500;
  let earsBreathePeriodMs = 1500;
  let earsRainbowPeriodMs = 3000;
  let earsResult = "未実行";

  let cameraResolution = "qvga";
  let cameraQuality = 12;
  let cameraCaptureResult = "未実行";
  let cameraCaptureRecent = null; // パース済みの結果オブジェクト
  let cameraLastCaptureAt = "-";
  let cameraLatencyMs = 0;
  let timerId;

  const fmtMs = (v) => `${v ?? 0} ms`;

  async function fetchJSON(url, options = {}) {
    const res = await fetch(url, {
      headers: { "Content-Type": "application/json" },
      ...options
    });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      const errorMessage = typeof body?.error === "string"
        ? body.error
        : (body?.error?.message || `HTTP ${res.status}`);
      throw new Error(errorMessage);
    }
    return body;
  }

  async function postHardware(path, payload = {}) {
    return fetchJSON(path, {
      method: "POST",
      body: JSON.stringify(payload)
    });
  }

  async function getHardwareState() {
    hardwareStatus = "実行中...";
    try {
      const result = await fetchJSON("/api/tests/hardware/state");
      hardwareStatus = JSON.stringify(result, null, 2);
      hardwareStatusUpdatedAt = new Date().toLocaleTimeString("ja-JP");
      touchStateLabel = "取得要求を送信済み";
      await loadOverview();
    } catch (err) {
      hardwareStatus = `失敗: ${err.message}`;
    }
  }

  async function runSpeakerTest() {
    speakerTestResult = "実行中...";
    try {
      const result = await postHardware("/api/tests/hardware/audio/play", {
        tone_hz: Number(speakerToneHz),
        duration_ms: Number(speakerDurationMs),
        volume: Number(speakerVolume)
      });
      speakerTestResult = JSON.stringify(result, null, 2);
      await loadOverview();
    } catch (err) {
      speakerTestResult = `失敗: ${err.message}`;
    }
  }

  async function runMicTest() {
    micTestResult = "実行中...";
    try {
      const result = await postHardware("/api/tests/hardware/mic/start", {
        duration_ms: Number(micDurationMs),
        sample_rate_hz: Number(micSampleRateHz),
        frame_duration_ms: Number(micFrameDurationMs)
      });
      micTestResult = JSON.stringify(result, null, 2);
      touchStateLabel = "mic テスト開始要求を送信済み";
      await loadOverview();
    } catch (err) {
      micTestResult = `失敗: ${err.message}`;
    }
  }

  async function runServoMove() {
    servoResult = "実行中...";
    try {
      const payload = {
        command: "move",
        axis: servoAxis,
        speed: Number(servoSpeed)
      };
      if (servoAxis === "x" || servoAxis === "both") {
        payload.angle_x_deg = Number(servoXDeg);
      }
      if (servoAxis === "y" || servoAxis === "both") {
        payload.angle_y_deg = Number(servoYDeg);
      }
      const result = await postHardware("/api/tests/hardware/servo", payload);
      servoResult = JSON.stringify(result, null, 2);
    } catch (err) {
      servoResult = `失敗: ${err.message}`;
    }
  }

  async function runServoHome() {
    servoAxis = "both";
    servoXDeg = 0;
    servoYDeg = 0;
    await runServoMove();
  }

  async function runServoCalibrationGet() {
    servoResult = "実行中...";
    try {
      const result = await postHardware("/api/tests/hardware/servo", {
        command: "calibration_get"
      });
      servoResult = JSON.stringify(result, null, 2);
    } catch (err) {
      servoResult = `失敗: ${err.message}`;
    }
  }

  async function runServoCalibrationSet() {
    servoResult = "実行中...";
    try {
      const result = await postHardware("/api/tests/hardware/servo", {
        command: "calibration_set",
        axis: servoCalAxis,
        center_offset_deg: Number(servoCenterOffsetDeg),
        min_deg: Number(servoMinDeg),
        max_deg: Number(servoMaxDeg),
        invert: Boolean(servoInvert),
        speed_limit_deg_per_sec: Number(servoSpeedLimitDegPerSec),
        soft_start: Boolean(servoSoftStart),
        home_deg: Number(servoHomeDeg)
      });
      servoResult = JSON.stringify(result, null, 2);
    } catch (err) {
      servoResult = `失敗: ${err.message}`;
    }
  }

  async function runLedTest() {
    ledResult = "実行中...";
    try {
      const result = await postHardware("/api/tests/hardware/led", {
        mode: ledMode,
        color: ledColor,
        brightness: Number(ledBrightness),
        blink_interval_ms: Number(ledBlinkIntervalMs),
        breathe_period_ms: Number(ledBreathePeriodMs)
      });
      ledResult = JSON.stringify(result, null, 2);
    } catch (err) {
      ledResult = `失敗: ${err.message}`;
    }
  }

  async function runEarsTest() {
    earsResult = "実行中...";
    try {
      const result = await postHardware("/api/tests/hardware/ears", {
        mode: earsMode,
        color: earsColor,
        brightness: Number(earsBrightness),
        blink_interval_ms: Number(earsBlinkIntervalMs),
        breathe_period_ms: Number(earsBreathePeriodMs),
        rainbow_period_ms: Number(earsRainbowPeriodMs)
      });
      earsResult = JSON.stringify(result, null, 2);
    } catch (err) {
      earsResult = `失敗: ${err.message}`;
    }
  }

  async function runCameraCaptureTest() {
    cameraCaptureResult = "実行中...";
    cameraCaptureRecent = null;
    const startTime = Date.now();
    try {
      const result = await postHardware("/api/tests/hardware/camera/capture", {
        resolution: cameraResolution,
        quality: Number(cameraQuality)
      });
      const latency = Date.now() - startTime;
      cameraLatencyMs = latency;
      cameraCaptureRecent = result;
      cameraCaptureResult = JSON.stringify(result, null, 2);
      cameraLastCaptureAt = new Date().toLocaleTimeString("ja-JP");
    } catch (err) {
      cameraCaptureResult = `失敗: ${err.message}`;
      cameraCaptureRecent = null;
    }
  }

  async function loadOverview() {
    snapshot = await fetchJSON("/api/runtime/overview");
  }

  async function refreshOverviewNow() {
    overviewMessage = "更新中...";
    try {
      await loadOverview();
      overviewMessage = `更新しました (${new Date().toLocaleTimeString("ja-JP")})`;
    } catch (err) {
      overviewMessage = `更新失敗: ${err.message}`;
    }
  }

  async function loadSettings() {
    settings = await fetchJSON("/api/settings");
    settingsMessage = `設定最終更新: ${settings.updated_at || "-"}`;
  }

  async function loadLLMSettings() {
    llmSettings = await fetchJSON("/api/settings/llm");
    llmSettingsMessage = `LLM設定最終更新: ${llmSettings.updated_at || "-"}`;
  }

  async function saveSettings(event) {
    event.preventDefault();
    try {
      const updated = await fetchJSON("/api/settings", {
        method: "PUT",
        body: JSON.stringify(settings)
      });
      settings = updated;
      settingsMessage = `保存しました (${updated.updated_at})`;
    } catch (err) {
      settingsMessage = `保存失敗: ${err.message}`;
    }
  }

  async function saveLLMSettings(event) {
    event.preventDefault();
    try {
      const updated = await fetchJSON("/api/settings/llm", {
        method: "POST",
        body: JSON.stringify(llmSettings)
      });
      llmSettings = updated;
      llmSettingsMessage = `保存しました (${updated.updated_at})`;
    } catch (err) {
      llmSettingsMessage = `保存失敗: ${err.message}`;
    }
  }

  async function runPipelineTest() {
    pipelineTestResult = "実行中...";
    try {
      const result = await fetchJSON("/api/tests/pipeline", {
        method: "POST",
        body: JSON.stringify({})
      });
      pipelineTestResult = JSON.stringify(result, null, 2);
      await loadOverview();
    } catch (err) {
      pipelineTestResult = `失敗: ${err.message}`;
    }
  }

  async function runVoicevoxUITest() {
    voicevoxResult = "実行中...";
    voicevoxAudioSrc = "";
    try {
      const result = await fetchJSON("/api/tests/voicevox/ui", {
        method: "POST",
        body: JSON.stringify({
          text: voicevoxText,
          speaker: Number(voicevoxSpeaker) || 1
        })
      });
      voicevoxResult = JSON.stringify(result, null, 2);
      voicevoxAudioSrc = `data:${result.content_type || "audio/wav"};base64,${result.audio_base64}`;
    } catch (err) {
      voicevoxResult = `失敗: ${err.message}`;
    }
  }

  async function runVoicevoxStackchanTest() {
    stackchanResult = "実行中...";
    try {
      const result = await fetchJSON("/api/tests/voicevox/stackchan", {
        method: "POST",
        body: JSON.stringify({
          text: stackchanText,
          speaker: Number(stackchanSpeaker) || 1,
          expression: stackchanExpression,
          motion: stackchanMotion,
          chunk_version: stackchanChunkVersion
        })
      });
      stackchanResult = JSON.stringify(result, null, 2);
      await loadOverview();
    } catch (err) {
      stackchanResult = `失敗: ${err.message}`;
    }
  }

  async function runLLMUITest() {
    llmUIResult = "実行中...";
    try {
      const result = await fetchJSON("/api/tests/llm/ui", {
        method: "POST",
        body: JSON.stringify({
          text: llmUIText,
          persona_override: llmUIPersona
        })
      });
      llmUIResult = JSON.stringify(result, null, 2);
      await loadOverview();
    } catch (err) {
      llmUIResult = `失敗: ${err.message}`;
    }
  }

  async function runLLMStackchanTest() {
    llmStackchanResult = "実行中...";
    try {
      const result = await fetchJSON("/api/tests/llm/stackchan", {
        method: "POST",
        body: JSON.stringify({
          text: llmStackchanText,
          persona_override: llmStackchanPersona,
          speaker: Number(llmStackchanSpeaker) || 1,
          expression: llmStackchanExpression,
          motion: llmStackchanMotion,
          chunk_version: llmStackchanChunkVersion
        })
      });
      llmStackchanResult = JSON.stringify(result, null, 2);
      await loadOverview();
    } catch (err) {
      llmStackchanResult = `失敗: ${err.message}`;
    }
  }

  $: alerts = (() => {
    const list = [];
    if ((snapshot.pipeline?.total_latency_ms || 0) > thresholds.totalLatencyMs) {
      list.push({ type: "warn", text: `total_latency_ms が高いです (${snapshot.pipeline.total_latency_ms} ms)` });
    }
    if ((snapshot.playback?.decode_error_count || 0) >= thresholds.decodeErrorCount) {
      list.push({ type: "danger", text: `decode_error_count が閾値以上です (${snapshot.playback.decode_error_count})` });
    }
    if ((snapshot.playback?.output_error_count || 0) >= thresholds.outputErrorCount) {
      list.push({ type: "danger", text: `output_error_count が閾値以上です (${snapshot.playback.output_error_count})` });
    }
    if ((snapshot.connection?.status || "").toLowerCase() !== "connected") {
      list.push({ type: "warn", text: `接続状態が ${snapshot.connection?.status || "unknown"} です` });
    }
    return list;
  })();

  onMount(async () => {
    await Promise.all([loadOverview(), loadSettings(), loadLLMSettings()]);
    timerId = setInterval(() => {
      loadOverview().catch(() => undefined);
    }, 3000);
    return () => clearInterval(timerId);
  });
</script>

<div class="bg-shape bg-shape-a"></div>
<div class="bg-shape bg-shape-b"></div>

<main class="layout">
  <header class="hero">
    <h1>Stackchan Runtime Console</h1>
    <p>接続状態・再生状態・会話遅延・設定変更・疎通テストを 1 画面で確認します。</p>
    <div class="controls">
      <button class="btn btn-primary" on:click={refreshOverviewNow}>今すぐ更新</button>
      <span class="updated-at">更新: {snapshot.updated_at || "-"}</span>
    </div>
    <p class="message hero-message">{overviewMessage}</p>
  </header>

  <section class="grid">
    <article class="card">
      <h2>接続とセッション</h2>
      <dl>
        <div><dt>Status</dt><dd>{snapshot.connection?.status || "-"}</dd></div>
        <div><dt>session_id</dt><dd>{snapshot.connection?.session_id || "-"}</dd></div>
        <div><dt>接続回数</dt><dd>{snapshot.connection?.connection_count ?? 0}</dd></div>
        <div><dt>再接続回数</dt><dd>{snapshot.connection?.reconnect_count ?? 0}</dd></div>
        <div><dt>最終 heartbeat</dt><dd>{snapshot.connection?.last_heartbeat_at || "-"}</dd></div>
      </dl>
    </article>

    <article class="card">
      <h2>音声再生</h2>
      <dl>
        <div><dt>state</dt><dd>{snapshot.playback?.state || "-"}</dd></div>
        <div><dt>request_id</dt><dd>{snapshot.playback?.request_id || "-"}</dd></div>
        <div><dt>start latency</dt><dd>{fmtMs(snapshot.playback?.playback_start_latency_ms || 0)}</dd></div>
        <div><dt>duration</dt><dd>{fmtMs(snapshot.playback?.playback_duration_ms || 0)}</dd></div>
        <div><dt>decode errors</dt><dd>{snapshot.playback?.decode_error_count ?? 0}</dd></div>
        <div><dt>output errors</dt><dd>{snapshot.playback?.output_error_count ?? 0}</dd></div>
      </dl>
    </article>

    <article class="card">
      <h2>会話パイプライン</h2>
      <dl>
        <div><dt>stream_id</dt><dd>{snapshot.pipeline?.stream_id || "-"}</dd></div>
        <div><dt>request_id</dt><dd>{snapshot.pipeline?.request_id || "-"}</dd></div>
        <div><dt>queue wait</dt><dd>{fmtMs(snapshot.pipeline?.queue_wait_ms || 0)}</dd></div>
        <div><dt>stt</dt><dd>{fmtMs(snapshot.pipeline?.stt_latency_ms || 0)}</dd></div>
        <div><dt>llm</dt><dd>{fmtMs(snapshot.pipeline?.llm_latency_ms || 0)}</dd></div>
        <div><dt>tts</dt><dd>{fmtMs(snapshot.pipeline?.tts_latency_ms || 0)}</dd></div>
        <div><dt>total</dt><dd>{fmtMs(snapshot.pipeline?.total_latency_ms || 0)}</dd></div>
        <div><dt>LLM input tokens</dt><dd>{snapshot.pipeline?.llm_input_token_count ?? 0}</dd></div>
        <div><dt>LLM output tokens</dt><dd>{snapshot.pipeline?.llm_output_token_count ?? 0}</dd></div>
        <div><dt>LLM total tokens</dt><dd>{snapshot.pipeline?.llm_total_token_count ?? 0}</dd></div>
        <div><dt>context turns</dt><dd>{snapshot.pipeline?.llm_effective_turns_in_context ?? 0}</dd></div>
        <div><dt>TTS Buffer Status</dt><dd>{snapshot.pipeline?.tts_watermark_status || "-"}</dd></div>
        <div><dt>TTS Buffered</dt><dd>{fmtMs(snapshot.pipeline?.tts_buffered_ms || 0)}</dd></div>
        <div><dt>Low Water Events</dt><dd>{snapshot.pipeline?.tts_low_water_count ?? 0}</dd></div>
        <div><dt>High Water Drops</dt><dd>{snapshot.pipeline?.tts_high_water_drop_count ?? 0}</dd></div>
      </dl>
    </article>

    <article class="card">
      <h2>アバター同期</h2>
      <dl>
        <div><dt>expression</dt><dd>{snapshot.avatar?.expression || "-"}</dd></div>
        <div><dt>motion</dt><dd>{snapshot.avatar?.motion || "-"}</dd></div>
        <div><dt>lip sync</dt><dd>{snapshot.avatar?.lip_sync_level ?? 0}</dd></div>
        <div><dt>update interval</dt><dd>{fmtMs(snapshot.avatar?.lip_sync_update_interval_ms || 0)}</dd></div>
      </dl>
    </article>

    <article class="card">
      <h2>Hardware Overview</h2>
      {#if (snapshot.connection?.status || "").toLowerCase() !== "connected"}
        <p class="message">デバイス未接続です。接続後に state.report が表示されます。</p>
      {/if}
      <dl>
        <div><dt>last report</dt><dd>{snapshot.hardware?.last_report_at || "-"}</dd></div>
        <div><dt>request_id</dt><dd>{snapshot.hardware?.request_id || "-"}</dd></div>
        <div><dt>source</dt><dd>{snapshot.hardware?.source || "-"}</dd></div>
        <div><dt>firmware</dt><dd>{snapshot.hardware?.firmware_version || "-"}</dd></div>
        <div><dt>uptime</dt><dd>{fmtMs(snapshot.hardware?.uptime_ms || 0)}</dd></div>
        <div><dt>rssi</dt><dd>{snapshot.hardware?.rssi ?? "-"}</dd></div>
        <div><dt>free heap</dt><dd>{snapshot.hardware?.free_heap_bytes ?? 0}</dd></div>
        <div><dt>current x</dt><dd>{snapshot.hardware?.current_angle_x_deg ?? 0}</dd></div>
        <div><dt>current y</dt><dd>{snapshot.hardware?.current_angle_y_deg ?? 0}</dd></div>
        <div><dt>mic level</dt><dd>{snapshot.hardware?.mic_level ?? 0}</dd></div>
        <div><dt>speaker busy</dt><dd>{snapshot.hardware?.speaker_busy ? "yes" : "no"}</dd></div>
        <div><dt>camera available</dt><dd>{snapshot.hardware?.camera_available ? "yes" : "no"}</dd></div>
        <div><dt>calib x range</dt><dd>{snapshot.hardware?.calibration?.x?.min_deg ?? "-"} - {snapshot.hardware?.calibration?.x?.max_deg ?? "-"}</dd></div>
        <div><dt>calib y range</dt><dd>{snapshot.hardware?.calibration?.y?.min_deg ?? "-"} - {snapshot.hardware?.calibration?.y?.max_deg ?? "-"}</dd></div>
      </dl>
    </article>
  </section>

  <section class="panel-group">
    <section class="card panel">
      <h2>設定変更</h2>
      <form class="settings-form" on:submit={saveSettings}>
        <label>音量
          <input type="number" min="0" max="100" bind:value={settings.playback_volume} required />
        </label>
        <label>表情プリセット
          <select bind:value={settings.expression_preset}>
            <option value="neutral">neutral</option>
            <option value="happy">happy</option>
            <option value="sad">sad</option>
            <option value="surprised">surprised</option>
          </select>
        </label>
        <label>リップシンク感度
          <input type="number" min="0" max="2" step="0.1" bind:value={settings.lip_sync_sensitivity} required />
        </label>
        <label>リップシンク減衰
          <input type="number" min="0" max="1" step="0.1" bind:value={settings.lip_sync_damping} required />
        </label>
        <label class="checkbox">
          <input type="checkbox" bind:checked={settings.motion_enabled} />
          モーション有効化
        </label>
        <div class="controls">
          <button type="submit" class="btn btn-primary">設定を保存</button>
          <button type="button" class="btn" on:click={() => loadSettings().catch(() => undefined)}>設定再読込</button>
        </div>
        <p class="message">{settingsMessage}</p>
      </form>
    </section>

    <section class="card panel">
      <h2>LLM 設定</h2>
      <form class="settings-form" on:submit={saveLLMSettings}>
        <label>System Prompt
          <textarea rows="5" bind:value={llmSettings.system_prompt}></textarea>
        </label>
        <div class="controls">
          <button type="submit" class="btn btn-primary">LLM設定を保存</button>
          <button type="button" class="btn" on:click={() => loadLLMSettings().catch(() => undefined)}>再読込</button>
        </div>
        <p class="message">{llmSettingsMessage}</p>
      </form>
    </section>

    <section class="card panel">
      <h2>疎通テスト</h2>
      <p>STT/LLM/TTS の最小パイプラインを実行し、レイテンシを表示します。</p>
      <div class="controls">
        <button class="btn btn-primary" on:click={() => runPipelineTest().catch(() => undefined)}>疎通テスト実行</button>
      </div>
      <pre class="result">{pipelineTestResult}</pre>
    </section>

    <section class="card panel">
      <h2>Voicevox UI 単体テスト</h2>
      <p>Stackchan 非接続でも Voicevox 音声生成を確認できます。</p>
      <form class="settings-form" on:submit|preventDefault={() => runVoicevoxUITest().catch(() => undefined)}>
        <label>入力テキスト
          <textarea rows="3" bind:value={voicevoxText}></textarea>
        </label>
        <label>speaker
          <input type="number" min="1" bind:value={voicevoxSpeaker} />
        </label>
        <div class="controls">
          <button type="submit" class="btn btn-primary">Voicevox テスト実行</button>
          <button type="button" class="btn" on:click={() => { voicevoxAudioSrc = ""; voicevoxResult = "未実行"; }}>結果クリア</button>
        </div>
      </form>
      {#if voicevoxAudioSrc}
        <audio class="audio-preview" controls src={voicevoxAudioSrc}></audio>
      {/if}
      <pre class="result">{voicevoxResult}</pre>
    </section>

    <section class="card panel">
      <h2>LLM UI 単体テスト</h2>
      <p>OpenAI LLM の応答と token 使用量を WebUI から確認します。</p>
      <form class="settings-form" on:submit|preventDefault={() => runLLMUITest().catch(() => undefined)}>
        <label>入力テキスト
          <textarea rows="3" bind:value={llmUIText}></textarea>
        </label>
        <label>persona override（任意）
          <textarea rows="2" bind:value={llmUIPersona}></textarea>
        </label>
        <div class="controls">
          <button type="submit" class="btn btn-primary">LLM テスト実行</button>
          <button type="button" class="btn" on:click={() => { llmUIResult = "未実行"; }}>結果クリア</button>
        </div>
      </form>
      <pre class="result">{llmUIResult}</pre>
    </section>

    <section class="card panel">
      <h2>Voicevox Stackchan 連携テスト</h2>
      <p>接続中の Stackchan へ `tts.chunk + tts.end` を送信して再生を確認します。`chunk_version` を切り替えて比較できます。</p>
      <form class="settings-form" on:submit|preventDefault={() => runVoicevoxStackchanTest().catch(() => undefined)}>
        <label>入力テキスト
          <textarea rows="3" bind:value={stackchanText}></textarea>
        </label>
        <label>speaker
          <input type="number" min="1" bind:value={stackchanSpeaker} />
        </label>
        <label>expression
          <select bind:value={stackchanExpression}>
            <option value="neutral">neutral</option>
            <option value="happy">happy</option>
            <option value="sad">sad</option>
            <option value="surprised">surprised</option>
          </select>
        </label>
        <label>motion
          <select bind:value={stackchanMotion}>
            <option value="idle">idle</option>
            <option value="nod">nod</option>
            <option value="shake">shake</option>
          </select>
        </label>
        <label>chunk version
          <select bind:value={stackchanChunkVersion}>
            <option value="1.0">1.0 (legacy fixed-byte)</option>
            <option value="1.1">1.1 (frame-based)</option>
          </select>
        </label>
        <div class="controls">
          <button type="submit" class="btn btn-primary">Stackchan 連携テスト実行</button>
          <button type="button" class="btn" on:click={() => { stackchanResult = "未実行"; }}>結果クリア</button>
        </div>
      </form>
      <pre class="result">{stackchanResult}</pre>
    </section>

    <section class="card panel">
      <h2>LLM Stackchan 連携テスト</h2>
      <p>text → LLM → Voicevox → Stackchan 送信までを一括確認します。</p>
      <form class="settings-form" on:submit|preventDefault={() => runLLMStackchanTest().catch(() => undefined)}>
        <label>入力テキスト
          <textarea rows="3" bind:value={llmStackchanText}></textarea>
        </label>
        <label>persona override（任意）
          <textarea rows="2" bind:value={llmStackchanPersona}></textarea>
        </label>
        <label>speaker
          <input type="number" min="1" bind:value={llmStackchanSpeaker} />
        </label>
        <label>expression
          <select bind:value={llmStackchanExpression}>
            <option value="neutral">neutral</option>
            <option value="happy">happy</option>
            <option value="sad">sad</option>
            <option value="surprised">surprised</option>
          </select>
        </label>
        <label>motion
          <select bind:value={llmStackchanMotion}>
            <option value="idle">idle</option>
            <option value="nod">nod</option>
            <option value="shake">shake</option>
          </select>
        </label>
        <label>chunk version
          <select bind:value={llmStackchanChunkVersion}>
            <option value="1.0">1.0 (legacy fixed-byte)</option>
            <option value="1.1">1.1 (frame-based)</option>
          </select>
        </label>
        <div class="controls">
          <button type="submit" class="btn btn-primary">LLM 連携テスト実行</button>
          <button type="button" class="btn" on:click={() => { llmStackchanResult = "未実行"; }}>結果クリア</button>
        </div>
      </form>
      <pre class="result">{llmStackchanResult}</pre>
    </section>

    <section class="card panel">
      <h2>Hardware Test: Touch / Mic / Speaker</h2>
      <p>入力と音声出力を API 経由で診断します。</p>
      <div class="kv-row"><span>Touch state</span><strong>{touchStateLabel}</strong></div>
      <form class="settings-form" on:submit|preventDefault={() => runSpeakerTest().catch(() => undefined)}>
        <label>tone_hz
          <input type="number" min="100" max="3000" bind:value={speakerToneHz} />
        </label>
        <label>duration_ms
          <input type="number" min="100" max="5000" bind:value={speakerDurationMs} />
        </label>
        <label>volume
          <input type="number" min="0" max="1" step="0.1" bind:value={speakerVolume} />
        </label>
        <div class="controls">
          <button type="submit" class="btn btn-primary">テストトーン再生</button>
          <button type="button" class="btn" on:click={() => runMicTest().catch(() => undefined)}>mic テスト開始</button>
          <button type="button" class="btn" on:click={() => getHardwareState().catch(() => undefined)}>状態更新</button>
        </div>
      </form>
      <pre class="result">{speakerTestResult}</pre>
      <pre class="result">{micTestResult}</pre>
    </section>

    <section class="card panel">
      <h2>Hardware Test: Servo</h2>
      <p>X/Y 手動制御と校正値保存を診断します。</p>
      <form class="settings-form" on:submit|preventDefault={() => runServoMove().catch(() => undefined)}>
        <label>axis
          <select bind:value={servoAxis}>
            <option value="x">x</option>
            <option value="y">y</option>
            <option value="both">both</option>
          </select>
        </label>
        <label>X angle ({servoXDeg} deg)
          <input type="range" min="-45" max="45" step="1" bind:value={servoXDeg} />
        </label>
        <label>Y angle ({servoYDeg} deg)
          <input type="range" min="-45" max="45" step="1" bind:value={servoYDeg} />
        </label>
        <label>speed
          <input type="number" min="0.1" max="3" step="0.1" bind:value={servoSpeed} />
        </label>
        <div class="controls">
          <button type="submit" class="btn btn-primary">X/Y を移動</button>
          <button type="button" class="btn" on:click={() => runServoHome().catch(() => undefined)}>home へ戻す</button>
          <button type="button" class="btn" on:click={() => runServoCalibrationGet().catch(() => undefined)}>校正読出し</button>
        </div>
      </form>
      <div class="divider"></div>
      <form class="settings-form" on:submit|preventDefault={() => runServoCalibrationSet().catch(() => undefined)}>
        <label>calibration axis
          <select bind:value={servoCalAxis}>
            <option value="x">x</option>
            <option value="y">y</option>
          </select>
        </label>
        <label>center_offset_deg
          <input type="number" step="0.1" bind:value={servoCenterOffsetDeg} />
        </label>
        <label>min_deg
          <input type="number" step="1" bind:value={servoMinDeg} />
        </label>
        <label>max_deg
          <input type="number" step="1" bind:value={servoMaxDeg} />
        </label>
        <label>speed_limit_deg_per_sec
          <input type="number" min="1" max="360" step="1" bind:value={servoSpeedLimitDegPerSec} />
        </label>
        <label>home_deg
          <input type="number" step="1" bind:value={servoHomeDeg} />
        </label>
        <label class="checkbox">
          <input type="checkbox" bind:checked={servoInvert} />
          invert
        </label>
        <label class="checkbox">
          <input type="checkbox" bind:checked={servoSoftStart} />
          soft_start
        </label>
        <div class="controls">
          <button type="submit" class="btn btn-primary">校正を保存</button>
        </div>
      </form>
      <pre class="result">{servoResult}</pre>
    </section>

    <section class="card panel">
      <h2>Hardware Test: LED / Ears</h2>
      <p>M5GO LED と NECO MIMI の色・明るさ・パターンを診断します。</p>
      <form class="settings-form" on:submit|preventDefault={() => runLedTest().catch(() => undefined)}>
        <label>LED mode
          <select bind:value={ledMode}>
            <option value="off">off</option>
            <option value="solid">solid</option>
            <option value="blink">blink</option>
            <option value="breathe">breathe</option>
          </select>
        </label>
        <label>LED color (#RRGGBB)
          <input type="text" bind:value={ledColor} />
        </label>
        <label>LED brightness
          <input type="number" min="0" max="255" bind:value={ledBrightness} />
        </label>
        <label>LED blink_interval_ms
          <input type="number" min="100" max="5000" bind:value={ledBlinkIntervalMs} />
        </label>
        <label>LED breathe_period_ms
          <input type="number" min="200" max="10000" bind:value={ledBreathePeriodMs} />
        </label>
        <div class="controls">
          <button type="submit" class="btn btn-primary">LED 送信</button>
        </div>
      </form>
      <pre class="result">{ledResult}</pre>

      <div class="divider"></div>

      <form class="settings-form" on:submit|preventDefault={() => runEarsTest().catch(() => undefined)}>
        <label>Ears mode
          <select bind:value={earsMode}>
            <option value="off">off</option>
            <option value="solid">solid</option>
            <option value="blink">blink</option>
            <option value="breathe">breathe</option>
            <option value="rainbow">rainbow</option>
          </select>
        </label>
        <label>Ears color (#RRGGBB)
          <input type="text" bind:value={earsColor} />
        </label>
        <label>Ears brightness
          <input type="number" min="0" max="255" bind:value={earsBrightness} />
        </label>
        <label>Ears blink_interval_ms
          <input type="number" min="100" max="5000" bind:value={earsBlinkIntervalMs} />
        </label>
        <label>Ears breathe_period_ms
          <input type="number" min="200" max="10000" bind:value={earsBreathePeriodMs} />
        </label>
        <label>Ears rainbow_period_ms
          <input type="number" min="200" max="10000" bind:value={earsRainbowPeriodMs} />
        </label>
        <div class="controls">
          <button type="submit" class="btn btn-primary">Ears 送信</button>
        </div>
      </form>
      <pre class="result">{earsResult}</pre>
    </section>

    <section class="card panel">
      <h2>Hardware Test: Camera / State</h2>
      <p>静止画取得要求とハードウェア状態要求を送信します。</p>
      <form class="settings-form" on:submit|preventDefault={() => runCameraCaptureTest().catch(() => undefined)}>
        <label>resolution
          <select bind:value={cameraResolution}>
            <option value="qqvga">qqvga</option>
            <option value="qvga">qvga</option>
            <option value="vga">vga</option>
          </select>
        </label>
        <label>quality
          <input type="number" min="1" max="63" bind:value={cameraQuality} />
        </label>
        <div class="controls">
          <button type="submit" class="btn btn-primary">静止画取得</button>
          <button type="button" class="btn" on:click={() => getHardwareState().catch(() => undefined)}>state.report 要求</button>
          <button type="button" class="btn" on:click={() => { cameraCaptureRecent = null; cameraCaptureResult = "未実行"; }}>結果クリア</button>
        </div>
      </form>

      {#if cameraCaptureRecent}
        <div class="camera-result">
          {#if cameraCaptureRecent.result?.ok}
            <div class="result-success">
              <h3>✅ 撮影成功</h3>
              
              {#if cameraCaptureRecent.result?.image_base64}
                <div class="camera-preview">
                  <img 
                    src="data:image/jpeg;base64,{cameraCaptureRecent.result.image_base64}" 
                    alt="Camera Capture Preview"
                    class="preview-image"
                  />
                </div>
              {/if}

              <div class="metadata-table">
                <h4>撮影情報</h4>
                <table>
                  <tbody>
                    <tr>
                      <td class="label">Request ID</td>
                      <td>{cameraCaptureRecent.request_id || "-"}</td>
                    </tr>
                    <tr>
                      <td class="label">Capture ID</td>
                      <td>{cameraCaptureRecent.result?.capture_id || "-"}</td>
                    </tr>
                    <tr>
                      <td class="label">時刻（device uptime）</td>
                      <td>{cameraCaptureRecent.result?.captured_at_ms || "-"} ms</td>
                    </tr>
                    <tr>
                      <td class="label">API 遅延</td>
                      <td>{cameraLatencyMs} ms</td>
                    </tr>
                  </tbody>
                </table>
              </div>

              <div class="metadata-table">
                <h4>画像情報</h4>
                <table>
                  <tbody>
                    <tr>
                      <td class="label">解像度（要求）</td>
                      <td>{cameraCaptureRecent.result?.requested_resolution || "-"}</td>
                    </tr>
                    <tr>
                      <td class="label">品質（要求）</td>
                      <td>{cameraCaptureRecent.result?.requested_quality || "-"}</td>
                    </tr>
                    <tr>
                      <td class="label">実寸法</td>
                      <td>{cameraCaptureRecent.result?.width || "?"} × {cameraCaptureRecent.result?.height || "?"} px</td>
                    </tr>
                    <tr>
                      <td class="label">データサイズ</td>
                      <td>{cameraCaptureRecent.result?.image_bytes || "-"} bytes</td>
                    </tr>
                    <tr>
                      <td class="label">カメラ利用可</td>
                      <td>{cameraCaptureRecent.result?.camera_available ? "✅ Yes" : "❌ No"}</td>
                    </tr>
                  </tbody>
                </table>
              </div>

              <p class="message">📸 最終撮影: {cameraLastCaptureAt}</p>
            </div>
          {:else}
            <div class="result-error">
              <h3>❌ 撮影失敗</h3>
              <p class="error-reason">
                <strong>理由：</strong> {cameraCaptureRecent.result?.reason || "不明なエラー"}
              </p>
              <div class="metadata-table">
                <h4>詳細情報</h4>
                <table>
                  <tbody>
                    <tr>
                      <td class="label">Request ID</td>
                      <td>{cameraCaptureRecent.request_id || "-"}</td>
                    </tr>
                    <tr>
                      <td class="label">API 遅延</td>
                      <td>{cameraLatencyMs} ms</td>
                    </tr>
                    <tr>
                      <td class="label">カメラ利用可</td>
                      <td>{cameraCaptureRecent.result?.camera_available ? "✅ Yes" : "❌ No"}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          {/if}
        </div>
      {/if}

      <p class="message">最終撮影要求: {cameraLastCaptureAt}</p>
      <p class="message">最終状態更新: {hardwareStatusUpdatedAt}</p>
      
      {#if cameraCaptureResult !== "未実行" && !cameraCaptureRecent}
        <div class="result-json-section">
          <details>
            <summary>🔧 JSON 生データを表示</summary>
            <pre class="result">{cameraCaptureResult}</pre>
          </details>
        </div>
      {/if}

      {#if hardwareStatus !== "未実行"}
        <div class="result-json-section">
          <details>
            <summary>🔧 ハードウェア状態を表示</summary>
            <pre class="result">{hardwareStatus}</pre>
          </details>
        </div>
      {/if}
    </section>
  </section>

  <section class="alerts">
    {#each alerts as item}
      <div class={`alert alert-${item.type}`}>{item.text}</div>
    {/each}
  </section>
</main>
