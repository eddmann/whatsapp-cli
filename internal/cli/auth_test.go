package cli

import (
	"context"
	"testing"
	"time"
)

func TestWaitForHistoryIdleResetsOnActivity(t *testing.T) {
	activity := make(chan struct{}, 1)
	go func() {
		for i := 0; i < 3; i++ {
			time.Sleep(30 * time.Millisecond)
			activity <- struct{}{}
		}
	}()

	start := time.Now()
	waitForHistoryIdle(context.Background(), activity, 60*time.Millisecond, time.Second)
	if elapsed := time.Since(start); elapsed < 120*time.Millisecond {
		t.Fatalf("expected activity to extend the wait, returned after %s", elapsed)
	}
}

func TestWaitForHistoryIdleHonorsMaxWaitAndCancel(t *testing.T) {
	activity := make(chan struct{})
	start := time.Now()
	waitForHistoryIdle(context.Background(), activity, time.Hour, 50*time.Millisecond)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("expected max wait to stop the wait, took %s", elapsed)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	waitForHistoryIdle(ctx, activity, time.Hour, time.Hour)
}
