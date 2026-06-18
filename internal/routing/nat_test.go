// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package routing

import (
	"fmt"
	"testing"
)

func TestAllocateOctetUniqueness(t *testing.T) {
	// Reset shared state so the test is hermetic regardless of run order.
	octetMu.Lock()
	usedOctets = make(map[int]bool)
	octetMu.Unlock()

	const n = 50
	seen := make(map[int]bool, n)
	for i := range n {
		got := allocateOctet(fmt.Sprintf("net-%d", i))
		if got < 10 || got > 210 {
			t.Errorf("octet %d outside [10,210] range", got)
		}
		if seen[got] {
			t.Errorf("duplicate octet %d allocated for distinct id net-%d", got, i)
		}
		seen[got] = true
	}
}

func TestAllocateOctetReleaseAndReuse(t *testing.T) {
	octetMu.Lock()
	usedOctets = make(map[int]bool)
	octetMu.Unlock()

	first := allocateOctet("session-A")
	releaseOctet(first)

	// After release the same id should yield the same hash-derived octet.
	second := allocateOctet("session-A")
	if second != first {
		t.Errorf("expected same octet after release+realloc of same id; got %d then %d", first, second)
	}
}

func TestAllocateOctetCollisionIncrement(t *testing.T) {
	octetMu.Lock()
	usedOctets = make(map[int]bool)
	octetMu.Unlock()

	first := allocateOctet("id-1")
	releaseOctet(first)

	// Pre-claim the octet "id-1" hashes to, forcing the increment path.
	octetMu.Lock()
	usedOctets[first] = true
	octetMu.Unlock()

	collided := allocateOctet("id-1")
	if collided == first {
		t.Errorf("expected collision walk to assign a different octet; got %d twice", first)
	}
	if collided < 10 || collided > 210 {
		t.Errorf("collision-walked octet %d outside valid range", collided)
	}
}
