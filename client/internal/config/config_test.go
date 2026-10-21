package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRejectsDuplicates(t *testing.T) {
	cfg := defaultConfig()
	cfg.Sliders[0] = Slider{DeviceID: "dev-a"}
	cfg.Sliders[1] = Slider{DeviceID: "dev-a"}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected a duplicate")
	}
	cfg.Sliders[1] = Slider{DeviceID: DefaultDeviceID}
	cfg.Sliders[2] = Slider{}
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Sliders[3] = Slider{DeviceID: DefaultDeviceID}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected a duplicate default device")
	}
}

func TestLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)

	store, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := store.Get()
	if cfg.ComPort != "auto" || !cfg.Logging || len(cfg.Sliders) != SliderCount {
		t.Fatalf("defaults: %+v", cfg)
	}

	cfg.InvertSliders = true
	cfg.Logging = false
	cfg.Sliders[0] = Slider{DeviceID: "dev-a", DeviceName: "Speakers"}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if _, gen := store.Snapshot(); gen != 1 {
		t.Fatalf("gen = %d", gen)
	}
	if err := store.RememberPort("COM7"); err != nil {
		t.Fatal(err)
	}
	if _, gen := store.Snapshot(); gen != 1 {
		t.Fatalf("remember must not increase gen, got %d", gen)
	}

	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	got := again.Get()
	if !got.InvertSliders || got.Logging || got.Sliders[0].DeviceID != "dev-a" || got.LastPort != "COM7" {
		t.Fatalf("loaded: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, appDirName, fileName)); err != nil {
		t.Fatal(err)
	}
}

func TestMissingLoggingStaysOn(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	path := filepath.Join(dir, appDirName)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("{\n  \"com_port\": \"auto\",\n  \"invert_sliders\": false,\n  \"autostart\": false\n}\n")
	if err := os.WriteFile(filepath.Join(path, fileName), body, 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !store.Get().Logging {
		t.Fatal("missing logging must stay on")
	}
}
