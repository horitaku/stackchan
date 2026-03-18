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
  let timerId;

  const fmtMs = (v) => `${v ?? 0} ms`;

  async function fetchJSON(url, options = {}) {
    const res = await fetch(url, {
      headers: { "Content-Type": "application/json" },
      ...options
    });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(body.error || `HTTP ${res.status}`);
    }
    return body;
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
    await Promise.all([loadOverview(), loadSettings()]);
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
  </section>

  <section class="alerts">
    {#each alerts as item}
      <div class={`alert alert-${item.type}`}>{item.text}</div>
    {/each}
  </section>
</main>
