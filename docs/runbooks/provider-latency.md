# 障害復旧ランブック: Provider 遅延・タイムアウト

## 概要

STT（音声認識）・LLM（言語モデル）・TTS（音声合成）などの外部 Provider が遅延またはタイムアウトした場合の診断・復旧手順を記載します。

---

## 1. 症状の分類

| 症状 | 原因候補 | 対応セクション |
|---|---|---|
| 発話が認識されない / 無反応 | STT タイムアウト、API キー不正 | [§2 STT（音声認識）の遅延](#2-stt音声認識の遅延) |
| 認識はされるが返答が来ない | LLM タイムアウト、レート制限 | [§3 LLM（言語モデル）の遅延](#3-llm言語モデルの遅延) |
| 返答テキストはあるが音声が出ない | TTS タイムアウト、Voicevox 停止 | [§4 TTS（音声合成）の遅延](#4-tts音声合成の遅延) |
| 全 Provider が一時的に失敗する | ネットワーク断、タイムアウト設定不足 | [§5 全体タイムアウト調整](#5-全体タイムアウト調整) |
| エラーが繰り返されてデバイス側でエラーが出る | リトライ回数超過 | [§6 リトライ設定の調整](#6-リトライ設定の調整) |

---

## 2. STT（音声認識）の遅延

### 診断

```bash
# サーバーログで STT エラーを確認
docker compose -f infra/docker/docker-compose.yml logs stackchan-server | grep -i "stt\|transcribe"

# runtime overview で STT レイテンシを確認
curl -fsS http://localhost:8080/api/runtime/overview | jq '.pipeline'
# 出力例:
# {
#   "stt_latency_ms": 4200,   ← 遅延が大きい場合は要確認
#   "llm_latency_ms": 1800,
#   "tts_latency_ms": 3100,
#   "total_latency_ms": 9100
# }

# 最近のメトリクス履歴を確認
curl -fsS "http://localhost:8080/api/runtime/metrics?metric_name=stt_latency_ms&limit=10" | jq .
```

### 確認事項

| 確認項目 | コマンド / 設定 |
|---|---|
| `OPENAI_API_KEY` が有効か | OpenAI ダッシュボードで使用量・ステータス確認 |
| `OPENAI_MODEL_STT` のモデル名が正しいか | `.env` の設定値、OpenAI API のモデル一覧と照合 |
| OpenAI API のステータスページ | [status.openai.com](https://status.openai.com)（外部リンク） |
| `PROVIDER_TIMEOUT_MS` が音声ファイルサイズに対して短すぎないか | `.env` のデフォルト 3000ms |

### 復旧手順

```bash
# 1. STT の単体テストで疎通確認
curl -fsS -X POST http://localhost:8080/api/tests/pipeline \
  -H "Content-Type: application/json" \
  -d '{"text": "テスト音声", "test_stt": true}' | jq .

# 2. タイムアウトを延長して再試行（.env を編集）
# PROVIDER_TIMEOUT_MS=8000  ← 8 秒に延長
# 変更後にサーバー再起動
mise run infra:down && mise run infra:up

# 3. API キーの残高・レート制限を確認
#    OpenAI ダッシュボード: https://platform.openai.com/usage
#    レート制限エラー (429) はログに出る:
#    [ERROR] STT failed error="rate limit exceeded" retryable=true
```

### エラーコードの意味

| `providers.Error.Code` | 状況 | 対処 |
|---|---|---|
| `timeout` | API 応答が `PROVIDER_TIMEOUT_MS` を超過 | タイムアウト延長または音声データ削減 |
| `temporary` | 一時的なサーバーエラー（5xx） | 自動リトライ済み。頻発なら API ステータス確認 |
| `internal` | API 認証エラー、不正な入力 | API キー・モデル名を確認 |

---

## 3. LLM（言語モデル）の遅延

### 診断

```bash
# LLM レイテンシを確認
curl -fsS http://localhost:8080/api/runtime/overview | jq '.pipeline.llm_latency_ms'

# LLM エラーのログを確認
docker compose -f infra/docker/docker-compose.yml logs stackchan-server | grep -i "llm\|generate\|chat"

# メトリクス履歴で傾向を把握
curl -fsS "http://localhost:8080/api/runtime/metrics?metric_name=llm_latency_ms&limit=20" | jq '.metrics[] | {created_at, value}'
```

### 確認事項

| 確認項目 | コマンド / 設定 |
|---|---|
| `OPENAI_MODEL_CHAT` が適切なモデルか | `.env` の設定 |
| プロンプト長が異常に長くなっていないか | `persona` や `memory` 設定の見直し |
| レート制限（TPM/RPM）に達していないか | OpenAI ダッシュボード |

### 復旧手順

```bash
# 1. LLM の単体テストを実行
curl -fsS -X POST http://localhost:8080/api/tests/pipeline \
  -H "Content-Type: application/json" \
  -d '{"text": "こんにちは", "test_llm": true}' | jq .

# 2. より軽量なモデルへの切り替え（高負荷時の応急処置）
#    .env を編集:
#    OPENAI_MODEL_CHAT=gpt-4o-mini  ← 軽量モデルへ変更

# 3. タイムアウトを延長
#    PROVIDER_TIMEOUT_MS=10000  ← 10 秒に延長
#    サーバー再起動
mise run infra:down && mise run infra:up
```

---

## 4. TTS（音声合成）の遅延

TTS は Voicevox（ローカル）と OpenAI TTS の 2 系統があります。現在の構成では Voicevox を優先しています。

### 4.1 Voicevox の診断

```bash
# Voicevox コンテナの起動確認
docker compose -f infra/docker/docker-compose.yml ps voicevox

# Voicevox API の疎通確認
curl -fsS http://localhost:50021/version
# 期待値: "latest" または バージョン文字列

# Voicevox 単体テスト（WebUI API 経由）
curl -fsS -X POST http://localhost:8080/api/tests/voicevox/ui \
  -H "Content-Type: application/json" \
  -d '{"text": "テストです", "speaker": 1}' | jq '{success: .success, latency_ms: .latency_ms}'
```

### 4.2 Voicevox の復旧手順

```bash
# Voicevox コンテナが停止している場合
docker compose -f infra/docker/docker-compose.yml restart voicevox

# 起動完了まで待機（CPU モデルは初回起動に 30〜60 秒かかることがある）
sleep 30 && curl -fsS http://localhost:50021/version

# コンテナのログでエラーを確認
docker compose -f infra/docker/docker-compose.yml logs voicevox --tail=50

# サーバー側の VOICEVOX_BASE_URL が正しいか確認
curl -fsS http://localhost:8080/api/settings | jq '.voicevox_base_url'
```

### 4.3 TTS レイテンシの確認

```bash
# TTS レイテンシの履歴
curl -fsS "http://localhost:8080/api/runtime/metrics?metric_name=tts_latency_ms&limit=10" | jq '.metrics[] | {created_at, value}'

# TTS 全体のサーバーログ
docker compose -f infra/docker/docker-compose.yml logs stackchan-server | grep -i "tts\|synth\|voicevox"
```

### 4.4 Voicevox の既知の遅延パターン

| 状況 | 典型的なレイテンシ | 対処 |
|---|---|---|
| 初回合成（モデルロード後） | 1,000〜3,000ms | `PROVIDER_TIMEOUT_MS` を 5,000ms 以上に設定 |
| 通常合成 | 300〜800ms | 問題なし |
| テキストが 200 文字以上 | 1,000ms〜 | テキストを分割して送信 |
| CPU モデルでの重負荷 | 2,000ms〜 | GPU モデルへの移行を検討 |

---

## 5. 全体タイムアウト調整

Provider タイムアウトに関連する環境変数の調整手順です。

### 現在のデフォルト設定

| 変数名 | デフォルト | 推奨調整値 |
|---|---|---|
| `PROVIDER_TIMEOUT_MS` | `3000` | `5000〜10000`（Voicevox CPU モード使用時） |
| `PROVIDER_MAX_ATTEMPTS` | `2` | `2〜3`（ネットワーク不安定時は 3） |
| `PROVIDER_RETRY_BASE_DELAY_MS` | `100` | `100〜500`（レート制限対策時は大きくする） |

### 調整手順

```bash
# 1. .env の現在値を確認
grep "PROVIDER_" .env

# 2. タイムアウトを調整（例: 5 秒に延長）
# .env を編集:
# PROVIDER_TIMEOUT_MS=5000
# PROVIDER_MAX_ATTEMPTS=3
# PROVIDER_RETRY_BASE_DELAY_MS=200

# 3. サーバーを再起動して反映
mise run infra:down && mise run infra:up

# 4. パイプラインテストで全 Provider の疎通を確認
curl -fsS -X POST http://localhost:8080/api/tests/pipeline \
  -H "Content-Type: application/json" \
  -d '{"text": "動作確認テスト"}' | jq .
```

---

## 6. リトライ設定の調整

### バックオフの動作確認

Provider のリトライは指数バックオフで実装されています。

```
試行 1: timeout → BaseDelay × 2^0 = 100ms 待機
試行 2: timeout → BaseDelay × 2^1 = 200ms 待機
試行 N: MaxAttempts に達したらエラー返却
```

デバイス側は `provider_timeout` / `provider_unavailable` エラーコードを受信します。

```bash
# エラーコード別のログ確認
docker compose -f infra/docker/docker-compose.yml logs stackchan-server \
  | grep -E "provider_timeout|provider_unavailable|provider_failed"
```

### リトライ頻発時の確認フロー

```bash
# 1. どの Provider でエラーが多いかを確認
docker compose -f infra/docker/docker-compose.yml logs stackchan-server \
  | grep -E "stt|llm|tts" | grep -i "error\|timeout\|failed"

# 2. メトリクス履歴で発生時刻帯を特定
curl -fsS "http://localhost:8080/api/runtime/metrics?metric_name=total_latency_ms&limit=50" \
  | jq '.metrics[] | select(.value > 10000) | {created_at, value}'

# 3. 該当 Provider のタイムアウトを個別に延長
#    （現在は PROVIDER_TIMEOUT_MS が全 Provider 共通のため、全体に反映される）
```

---

## 7. Provider が完全に使えない場合の代替措置

| Provider | 代替策 |
|---|---|
| STT (OpenAI) | テキスト入力モードへの切り替え（WebUI のパイプラインテスト経由） |
| LLM (OpenAI) | モデルを軽量版（`gpt-4o-mini`）に変更 |
| TTS (Voicevox) | OpenAI TTS provider に切り替え（providers 設定を変更） |

---

## 8. エスカレーション基準

| 状況 | 対応 |
|---|---|
| 特定 Provider のエラー率が 50% を超えている | API キー・サービスステータスを確認し、開発チームに共有 |
| タイムアウト調整後も改善しない | Provider のソース実装（`server/internal/providers/`）を確認 |
| Voicevox が起動しない | Docker ログを確認し、コンテナイメージの再プルを検討 |
| OpenAI API キーの有効期限切れ | 新しいキーを発行して `.env` を更新後、サーバー再起動 |
