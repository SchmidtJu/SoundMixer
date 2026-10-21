package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFailedSaveDoesNotPublishConfiguration(t *testing.T) {
	for _, failure := range []string{"write", "rename"} {
		t.Run(failure, func(t *testing.T) {
			t.Setenv("APPDATA", t.TempDir())
			store, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			before, generation := store.Snapshot()
			if failure == "write" {
				// A directory at the temporary file path makes WriteFile fail.
				if err := os.Mkdir(store.path+".tmp", 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				// A nonempty destination directory makes Rename fail.
				store.path = filepath.Join(filepath.Dir(store.path), "blocked")
				if err := os.Mkdir(store.path, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(store.path, "keep"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			next := store.Get()
			next.InvertSliders = true
			next.Sliders[0] = Slider{DeviceID: "new-device"}
			if err := store.Save(next); err == nil {
				t.Fatal("expected save failure")
			}
			if err := store.RememberPort("COM99"); err == nil {
				t.Fatal("expected remember failure")
			}
			after, gotGeneration := store.Snapshot()
			if !reflect.DeepEqual(before, after) || gotGeneration != generation {
				t.Fatalf("failed write changed live state: %+v, generation=%d", after, gotGeneration)
			}
		})
	}
}

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
	cfg.UpdateIntervalMs = 250
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
	if !got.InvertSliders || got.Logging || got.UpdateIntervalMs != 250 || got.Sliders[0].DeviceID != "dev-a" || got.LastPort != "COM7" {
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
	if store.Get().UpdateIntervalMs != DefaultUpdateIntervalMs {
		t.Fatal("old configs must use the default update interval")
	}
}

func TestUpdateIntervalValidation(t *testing.T) {
	for _, ms := range []int{-1, 19, 2001} {
		cfg := defaultConfig()
		cfg.UpdateIntervalMs = ms
		if err := Validate(cfg); err == nil {
			t.Fatalf("accepted invalid interval %d", ms)
		}
		normalize(&cfg)
		if cfg.UpdateIntervalMs != DefaultUpdateIntervalMs {
			t.Fatalf("invalid stored interval %d did not fall back to default", ms)
		}
	}
	for _, ms := range []int{0, 20, 100, 500, 2000} {
		cfg := defaultConfig()
		cfg.UpdateIntervalMs = ms
		if err := Validate(cfg); err != nil {
			t.Fatalf("rejected interval %d: %v", ms, err)
		}
	}
}
