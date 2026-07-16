package chromedp

import (
	"testing"
	"time"
)

// TestWaitReadyOriginalProblem reproduces the exact problem described in the bug report.
//
// This test demonstrates that chromedp.WaitReady() previously always timed out
// even for basic HTML elements that are clearly present in the DOM.
//
// Before the fix: This test would ALWAYS fail with "context deadline exceeded"
// After the fix: This test should PASS consistently
func TestWaitReadyOriginalProblem(t *testing.T) {
	ctx, cancel := testAllocate(t, "")
	defer cancel()

	startTime := time.Now()

	err := Run(ctx,
		Navigate("data:text/html,<html><body>Hello</body></html>"),
		WaitReady("html", ByQuery), // This used to ALWAYS timeout
	)

	duration := time.Since(startTime)

	if err != nil {
		t.Fatalf("WaitReady failed with: %v (duration: %v)", err, duration)
	}

	t.Logf("✅ WaitReady succeeded in %v", duration)

	// Verify it completed much faster than the timeout
	if duration > 2*time.Second {
		t.Logf("⚠️  Warning: took %v, expected much faster", duration)
	}
}

// TestWaitReadyComparison compares WaitReady with WaitVisible to show the fix works
func TestWaitReadyComparison(t *testing.T) {
	ctx, cancel := testAllocate(t, "")
	defer cancel()

	testHTML := "data:text/html,<html><body><h1>Test</h1></body></html>"

	// Test WaitReady (our fix)
	start := time.Now()
	err := Run(ctx,
		Navigate(testHTML),
		WaitReady("html", ByQuery),
	)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("WaitReady failed: %v (duration: %v)", err, duration)
	} else {
		t.Logf("✅ WaitReady succeeded in %v", duration)
	}

	// Test WaitVisible for comparison
	start = time.Now()
	err = Run(ctx,
		Navigate(testHTML),
		WaitVisible("html", ByQuery),
	)
	duration = time.Since(start)

	if err != nil {
		t.Errorf("WaitVisible failed: %v (duration: %v)", err, duration)
	} else {
		t.Logf("✅ WaitVisible succeeded in %v", duration)
	}
}

// TestMinimalReproduction is the absolute minimal test case
func TestMinimalReproduction(t *testing.T) {
	ctx, cancel := testAllocate(t, "")
	defer cancel()

	// The most basic test - just check if HTML element exists
	err := Run(ctx,
		Navigate("data:text/html,<html></html>"),
		WaitReady("html", ByQuery),
	)

	if err != nil {
		t.Fatalf("Minimal reproduction failed: %v", err)
	}

	t.Log("✅ Minimal test passed")
}