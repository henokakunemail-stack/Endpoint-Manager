package update

import (
	"context"
	"strings"
	"testing"
)

// TestApplyUpdate_EmptyDownloadURLDoesNotPanic is a regression test for a crash
// that took the whole agent process down: ApplyUpdate indexed DownloadURL[0]
// without checking its length, so a rollout row with no artifact URL panicked
// on a background goroutine that had no recover(). An empty URL must produce an
// error instead.
func TestApplyUpdate_EmptyDownloadURLDoesNotPanic(t *testing.T) {
	engine := NewEngine("https://mgmt.example.com", "dev-1", "secret")

	// Any panic here fails the test, which is the point: the old code panicked
	// before it could report a result.
	err := engine.ApplyUpdate(context.Background(), UpdateParams{
		TaskID:        "task-1",
		TargetVersion: "1.2.3",
		DownloadURL:   "",
	})

	if err == nil {
		t.Fatal("expected an error for an empty download URL, got nil")
	}
	if !strings.Contains(err.Error(), "download_url") {
		t.Errorf("error = %q, want it to mention download_url", err)
	}
}
