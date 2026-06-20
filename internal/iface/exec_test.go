// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package iface

import (
	"testing"
	"time"
)

// A wedged radio makes iw block forever in the kernel. execCapture must bound it
// so the adapter scan (and the agent status path that drives it) can never hang.
func TestExecCaptureTimesOut(t *testing.T) {
	start := time.Now()
	_, err := execCapture("sleep", "30")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected a timeout error from a hung command, got nil")
	}
	if elapsed > 8*time.Second {
		t.Fatalf("execCapture did not honor its timeout: took %s", elapsed)
	}
}
