// Package protocol - sequence 管理を提供します。
// firmware->server / server->firmware の direction ごとに単調増加を保証します。
package protocol

import "sync"

// CheckResult は受信 sequence の検証結果を表します。
type CheckResult int

const (
	SequenceOK        CheckResult = iota // 正常
	SequenceDuplicate                    // 重複（呼び出し元はスキップすること）
	SequenceReversed                     // 逆転（warning を出すが処理は続ける）
)

// SequenceTracker はセッションごとの sequence 状態を管理します。
// インスタンスは New 経由で生成し、複数 goroutine から安全に使用できます。
type SequenceTracker struct {
	mu           sync.Mutex
	maxInbound   int64              // firmware->server で受信した最大 sequence
	nextOutbound int64              // 次に送信する server->firmware の sequence 番号
	seen         map[int64]struct{} // 重複検知用セット（TODO: 長期運用時は LRU 等に置き換える）
}

// NewSequenceTracker は SequenceTracker を初期化して返します。
func NewSequenceTracker() *SequenceTracker {
	return &SequenceTracker{
		nextOutbound: 1,
		seen:         make(map[int64]struct{}),
	}
}

// CheckInbound は firmware->server 方向の受信 sequence を検証します。
// SequenceDuplicate の場合は呼び出し元でメッセージをスキップしてください。
// SequenceReversed の場合は warning ログを出力しつつ処理を継続してください。
func (t *SequenceTracker) CheckInbound(seq int64) (CheckResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// sequence の下限チェック（Validate() で弾かれているはずだが二重確認）
	if seq < 1 {
		return SequenceOK, nil
	}

	// 重複チェック: 過去に受信済みなら再処理しない
	if _, exists := t.seen[seq]; exists {
		return SequenceDuplicate, nil
	}

	// 逆転チェック: 過去の最大値より小さければ順序逆転
	result := SequenceOK
	if t.maxInbound > 0 && seq < t.maxInbound {
		result = SequenceReversed
	}

	// 受信済みセットに追加し、最大値を更新します
	t.seen[seq] = struct{}{}
	if seq > t.maxInbound {
		t.maxInbound = seq
	}
	return result, nil
}

// NextOutbound は server->firmware 方向の次の sequence 番号を採番して返します。
// 呼び出しのたびに単調増加します。
func (t *SequenceTracker) NextOutbound() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	seq := t.nextOutbound
	t.nextOutbound++
	return seq
}
