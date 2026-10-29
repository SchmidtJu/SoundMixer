//go:build windows

package audio

import (
	"testing"
	"time"

	"soundmixer/internal/config"
)

func TestFormatLevels(t *testing.T) {
	var snap [config.SliderCount]sliderState
	snap[0].level = 0.23
	snap[1].level = 0.24
	snap[2].level = 0.57
	snap[3].level = 0.89
	got := formatLevels(snap)
	want := "1=23% 2=24% 3=57% 4=89%"
	if got != want {
		t.Fatalf("got %q", got)
	}
	if percent(0) != 0 || percent(1) != 100 || percent(0.995) != 100 {
		t.Fatalf("edges: %d %d %d", percent(0), percent(1), percent(0.995))
	}
}

func TestApplyDue(t *testing.T) {
	now := time.Unix(1_000, 0)
	if !applyDue(now, time.Time{}) {
		t.Fatal("the first write should apply immediately")
	}
	next := now.Add(100 * time.Millisecond)
	if applyDue(now.Add(99*time.Millisecond), next) {
		t.Fatal("a second write before 100 ms should not apply")
	}
	if !applyDue(now.Add(100*time.Millisecond), next) {
		t.Fatal("the stored level should apply at 100 ms")
	}
}

func TestUpdateIntervalReschedulesPendingWrite(t *testing.T) {
	c := New()
	if c.applyInterval != 100*time.Millisecond {
		t.Fatalf("default interval: %v", c.applyInterval)
	}
	now := time.Unix(1000, 0)
	c.lastApply = now
	c.dirty = true
	c.waiting = true
	for _, ms := range []int{500, 20, 2000, 100} {
		c.SetUpdateInterval(ms)
		want := now.Add(time.Duration(ms) * time.Millisecond)
		if !c.nextApply.Equal(want) || c.waiting || !c.dirty {
			t.Fatalf("interval %d: deadline=%v, waiting=%v, dirty=%v", ms, c.nextApply, c.waiting, c.dirty)
		}
		if applyDue(want.Add(-time.Millisecond), c.nextApply) || !applyDue(want, c.nextApply) {
			t.Fatalf("incorrect deadline at %d ms", ms)
		}
		select {
		case <-c.wake:
		default:
			t.Fatal("interval change must wake the controller")
		}
	}
	c.SetUpdateInterval(0)
	if c.applyInterval != 100*time.Millisecond {
		t.Fatal("invalid intervals must fall back to the default")
	}
}
