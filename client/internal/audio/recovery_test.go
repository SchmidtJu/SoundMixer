//go:build windows

package audio

import (
	"errors"
	"testing"

	"soundmixer/internal/config"
)

func TestRetryFailedVolumeAndSkipSuccessfulDuplicates(t *testing.T) {
	c := New()
	var snap [config.SliderCount]sliderState
	snap[0] = sliderState{deviceID: "speakers", level: 0.5, active: true}
	calls := 0
	set := func(string, float32) error {
		calls++
		if calls == 1 {
			return errors.New("device disconnected")
		}
		return nil
	}
	resolve := func() (string, error) { return "", nil }
	c.applyTargets(snap, resolve, set)
	if c.last[0].ok {
		t.Fatal("failed volume must remain pending")
	}
	c.applyTargets(snap, resolve, set)
	c.applyTargets(snap, resolve, set)
	if calls != 2 || !c.last[0].ok {
		t.Fatalf("calls=%d, applied=%v", calls, c.last[0].ok)
	}
}

func TestDefaultDeviceChangeAndReconnect(t *testing.T) {
	c := New()
	var snap [config.SliderCount]sliderState
	snap[0] = sliderState{deviceID: config.DefaultDeviceID, level: 0.5, active: true}
	target := "speakers"
	resolve := func() (string, error) { return target, nil }
	var writes []string
	set := func(id string, _ float32) error {
		writes = append(writes, id)
		return nil
	}
	c.applyTargets(snap, resolve, set)
	target = "headphones"
	c.applyTargets(snap, resolve, set)
	c.invalidateDisconnected(nil)
	c.applyTargets(snap, resolve, set)
	c.invalidateDisconnected([]Device{{ID: target}})
	c.applyTargets(snap, resolve, set)
	if len(writes) != 3 || writes[0] != "speakers" || writes[1] != target || writes[2] != target {
		t.Fatalf("unexpected writes: %v", writes)
	}
}
