//go:build windows

package audio

import (
	"testing"
	"time"
)

func TestListPlaybackDevices(t *testing.T) {
	ctrl := New()
	go ctrl.Run()
	defer ctrl.Stop()

	deadline := time.Now().Add(3 * time.Second)
	var (
		devices []Device
		err     error
	)
	for time.Now().Before(deadline) {
		devices, err = ctrl.Devices()
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) == 0 {
		t.Fatal("no playback devices")
	}
	for _, device := range devices {
		if device.ID == "" || device.Name == "" {
			t.Fatalf("incomplete device: %+v", device)
		}
		t.Logf("%s | %s", device.Name, device.ID)
	}
}
