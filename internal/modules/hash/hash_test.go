package hash

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCustomEpoch(t *testing.T) {
	baseDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	timestamp := time.Now().Unix()
	diff := timestamp - baseDate

	if diff < 0 || diff > (1<<timestampBits)-1 {
		t.Errorf("timestamp difference %d is out of the %d-bit budget", diff, timestampBits)
	}
}

func TestNewGenerator(t *testing.T) {

	cases := []struct {
		name    string
		nodeID  int64
		wantErr bool
	}{
		{"valid nodeID in the middle of the range", 7, false},
		{"valid nodeID at the lower bound", 0, false},
		{"valid nodeID at the upper bound", maxNodeID, false},
		{"nodeID above the maximum", maxNodeID + 1, true},
		{"nodeID way above the maximum", 999, true},
		{"negative nodeID", -5, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gen, err := NewGenerator(c.nodeID)

			if c.wantErr {
				if err == nil {
					t.Errorf("expected an error for nodeID=%d, got nil", c.nodeID)
				}
				if gen != nil {
					t.Errorf("expected a nil generator when there's an error, got %+v", gen)
				}
			} else {
				if err != nil {
					t.Errorf("did not expect an error for nodeID=%d, got: %v", c.nodeID, err)
				}
				if gen == nil {
					t.Fatalf("expected a non-nil generator for valid nodeID=%d", c.nodeID)
				}
				if gen.nodeID != c.nodeID {
					t.Errorf("stored nodeID = %d, expected %d", gen.nodeID, c.nodeID)
				}
			}
		})
	}
}

// --- currentTimestamp tests (TODO 5) ---

func TestCurrentTimestamp_AdvancesWithRealTime(t *testing.T) {

	g, err := NewGenerator(1)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	t1 := g.currentTimestamp()
	time.Sleep(2 * time.Second)
	t2 := g.currentTimestamp()

	if diff := t2 - t1; diff < 2 {
		t.Errorf("expected a difference >= 2s between readings, got %d", diff)
	}
}

func TestCurrentTimestamp_ConsistentAcrossNodes(t *testing.T) {

	g1, _ := NewGenerator(7)
	g2, _ := NewGenerator(42)

	if g1.currentTimestamp() != g2.currentTimestamp() {
		t.Error("two nodes sharing the same epoch should read the same timestamp at the same instant")
	}
}

// --- NextID tests (TODO 6) ---

func TestNextID_ClockMovedBackwards(t *testing.T) {

	g, _ := NewGenerator(1)

	// simulate a lastTimestamp "in the future" relative to the current clock
	g.lastTimestamp = g.currentTimestamp() + 100

	id, err := g.NextID()
	if err == nil {
		t.Error("expected an error when the clock appears to have moved backwards")
	}
	if id != 0 {
		t.Errorf("expected id=0 on the error path, got %d", id)
	}
}

func TestNextID_SequenceIncrementsWithinSameSecond(t *testing.T) {

	g, _ := NewGenerator(1)

	var lastSeq int64 = -1
	for i := 0; i < 5; i++ {
		id, err := g.NextID()
		if err != nil {
			t.Fatalf("call %d returned an unexpected error: %v", i, err)
		}
		if id == 0 {
			t.Fatalf("call %d returned an unexpected id=0", i)
		}
		if lastSeq >= 0 && g.sequence != lastSeq+1 {
			// this is only a real failure if we're still within the same second
			if g.lastTimestamp == g.currentTimestamp() {
				t.Errorf("call %d: sequence did not increment as expected (%d -> %d)", i, lastSeq, g.sequence)
			}
		}
		lastSeq = g.sequence
	}
}

func TestNextID_SequenceOverflowWaitsForNextSecond(t *testing.T) {
	g, _ := NewGenerator(1)

	now := g.currentTimestamp()
	g.lastTimestamp = now
	g.sequence = maxSequence // already at the limit: the next call must overflow

	start := time.Now()
	id, err := g.NextID()
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("did not expect an error on overflow, got: %v", err)
	}
	if id == 0 {
		t.Error("expected a valid id after waiting for the next second")
	}
	if g.sequence != 0 {
		t.Errorf("expected sequence=0 after overflow, got %d", g.sequence)
	}
	if g.lastTimestamp <= now {
		t.Errorf("expected lastTimestamp to advance to the next second, got %d (was %d)", g.lastTimestamp, now)
	}
	if elapsed > 1200*time.Millisecond {
		t.Errorf("expected to wait at most ~1s for the next second, took %v", elapsed)
	}
}

func TestNextID_NoCollisionsAcrossNodes(t *testing.T) {

	const numNodes = 10
	const idsPerNode = 200

	var mu sync.Mutex
	seen := make(map[int64]bool)
	var wg sync.WaitGroup

	for nodeID := int64(0); nodeID < numNodes; nodeID++ {
		wg.Add(1)
		go func(nodeID int64) {
			defer wg.Done()
			gen, err := NewGenerator(nodeID)
			if err != nil {
				t.Error(err)
				return
			}
			for i := 0; i < idsPerNode; i++ {
				id, err := gen.NextID()
				if err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				if seen[id] {
					t.Errorf("collision detected: id=%d", id)
				}
				seen[id] = true
				mu.Unlock()
			}
		}(nodeID)
	}

	wg.Wait()

	if len(seen) != numNodes*idsPerNode {
		t.Errorf("expected %d unique ids, got %d", numNodes*idsPerNode, len(seen))
	}
}

// --- EncodeBase62 tests (TODO 7) ---

func TestEncodeBase62(t *testing.T) {
	cases := []struct {
		name string
		id   int64
		want string
	}{
		{"zero should be fully zero-padded", 0, "0000000"},
		{"a single character at the end of the alphabet", 61, "000000Z"},
		{"exactly the base (rollover)", 62, "0000010"},
		{"small value with padding", 125, "0000021"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EncodeBase62(c.id)
			if got != c.want {
				t.Errorf("EncodeBase62(%d) = %q, expected %q", c.id, got, c.want)
			}
		})
	}
}

func TestEncodeBase62_AlwaysSevenCharacters(t *testing.T) {
	values := []int64{0, 1, 61, 62, 3844, 1<<41 - 1}
	for _, id := range values {
		got := EncodeBase62(id)
		if len(got) != urlLength {
			t.Errorf("EncodeBase62(%d) has length %d, expected %d (value: %q)", id, len(got), urlLength, got)
		}
	}
}

func TestEncodeBase62_OnlyValidAlphabetCharacters(t *testing.T) {
	got := EncodeBase62(427497584867)
	for _, r := range got {
		if !strings.ContainsRune(base62Alphabet, r) {
			t.Errorf("character %q is outside the expected base62 alphabet", r)
		}
	}
}