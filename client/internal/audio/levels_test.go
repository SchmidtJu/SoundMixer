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
	next := now.Add(applyInterval)
	if applyDue(now.Add(99*time.Millisecond), next) {
		t.Fatal("a second write before 100 ms should not apply")
	}
	if !applyDue(now.Add(100*time.Millisecond), next) {
		t.Fatal("the stored level should apply at 100 ms")
	}
}
