package host

import (
	"math"
	"testing"
	"time"
)

func TestRecordKeySuppressesImmediateDuplicate(t *testing.T) {
	monitor := &TypingMonitor{}

	monitor.recordKey(30)
	monitor.recordKey(30)

	if got := len(monitor.keystrokes); got != 1 {
		t.Fatalf("keystrokes = %d, want 1", got)
	}
}

func TestSnapshotUsesMinimumSpanToAvoidBurstSpikes(t *testing.T) {
	now := time.Now()
	monitor := &TypingMonitor{
		active:  true,
		lastKey: now,
	}
	for i := 0; i < 10; i++ {
		monitor.keystrokes = append(monitor.keystrokes, keystroke{
			at:   now.Add(-2*time.Second + time.Duration(i)*200*time.Millisecond),
			code: 30 + i,
		})
	}

	wpm, active := monitor.Snapshot(time.Second)
	if !active {
		t.Fatal("active = false, want true")
	}

	if math.Abs(wpm-20) > 0.1 {
		t.Fatalf("wpm = %.1f, want about 20.0", wpm)
	}
}
