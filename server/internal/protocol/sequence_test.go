package protocol_test

import (
	"testing"

	"github.com/stackchan/server/internal/protocol"
)

// TestSequenceTracker_NormalFlow は正常な単調増加シーケンスが SequenceOK を返すことを確認します。
func TestSequenceTracker_NormalFlow(t *testing.T) {
	tracker := protocol.NewSequenceTracker()

	seqs := []int64{1, 2, 3, 5, 10} // 欠番は許容する
	for _, seq := range seqs {
		result, err := tracker.CheckInbound(seq)
		if err != nil {
			t.Fatalf("seq=%d: unexpected error: %v", seq, err)
		}
		if result != protocol.SequenceOK {
			t.Errorf("seq=%d: expected SequenceOK, got %v", seq, result)
		}
	}
}

// TestSequenceTracker_Duplicate は同じ sequence を 2 回送るとスキップ指示が返ることを確認します。
func TestSequenceTracker_Duplicate(t *testing.T) {
	tracker := protocol.NewSequenceTracker()

	if _, err := tracker.CheckInbound(3); err != nil {
		t.Fatal(err)
	}
	result, err := tracker.CheckInbound(3) // 同じ番号を再送
	if err != nil {
		t.Fatal(err)
	}
	if result != protocol.SequenceDuplicate {
		t.Errorf("expected SequenceDuplicate, got %v", result)
	}
}

// TestSequenceTracker_Reversed は大きい番号の後に小さい番号が来ると SequenceReversed を返すことを確認します。
func TestSequenceTracker_Reversed(t *testing.T) {
	tracker := protocol.NewSequenceTracker()

	if _, err := tracker.CheckInbound(10); err != nil {
		t.Fatal(err)
	}
	result, err := tracker.CheckInbound(5) // 10 より小さい未受信の番号
	if err != nil {
		t.Fatal(err)
	}
	if result != protocol.SequenceReversed {
		t.Errorf("expected SequenceReversed, got %v", result)
	}
}

// TestSequenceTracker_ReversedThenNormal は逆転後に最大値より大きい番号が来ると正常に戻ることを確認します。
func TestSequenceTracker_ReversedThenNormal(t *testing.T) {
	tracker := protocol.NewSequenceTracker()

	tracker.CheckInbound(10) //nolint
	tracker.CheckInbound(5)  // reversed だが受け付ける //nolint

	result, err := tracker.CheckInbound(11) // maxInbound=10 より大きいので正常
	if err != nil {
		t.Fatal(err)
	}
	if result != protocol.SequenceOK {
		t.Errorf("expected SequenceOK after recovery, got %v", result)
	}
}

// TestSequenceTracker_Outbound は送信 sequence が 1 から単調増加することを確認します。
func TestSequenceTracker_Outbound(t *testing.T) {
	tracker := protocol.NewSequenceTracker()

	for i := int64(1); i <= 5; i++ {
		seq := tracker.NextOutbound()
		if seq != i {
			t.Errorf("expected outbound seq=%d, got %d", i, seq)
		}
	}
}
