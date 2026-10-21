package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	SliderCount     = 4
	DefaultDeviceID = "system.default"
	appDirName      = "SoundMixer"
	fileName        = "config.json"
)

// Slider maps one control to a Windows playback device.
// DeviceID is the stable endpoint ID. Empty means unassigned.
// DefaultDeviceID follows the current Windows default device.
type Slider struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

// Config is stored in %AppData%\SoundMixer\config.json.
type Config struct {
	ComPort       string   `json:"com_port"`
	LastPort      string   `json:"last_port"`
	InvertSliders bool     `json:"invert_sliders"`
	Autostart     bool     `json:"autostart"`
	Logging       bool     `json:"logging"`
	Sliders       []Slider `json:"sliders"`
}

// Store keeps the configuration in memory and on disk.
type Store struct {
	mu   sync.RWMutex
	cfg  Config
	path string
	gen  uint64
}

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName), nil
}

func defaultConfig() Config {
	return Config{
		ComPort: "auto",
		Logging: true,
		Sliders: make([]Slider, SliderCount),
	}
}

func normalize(cfg *Config) {
	if cfg.ComPort == "" {
		cfg.ComPort = "auto"
	}
	if len(cfg.Sliders) > SliderCount {
		cfg.Sliders = cfg.Sliders[:SliderCount]
	}
	for len(cfg.Sliders) < SliderCount {
		cfg.Sliders = append(cfg.Sliders, Slider{})
	}
}

// Validate rejects duplicate device assignments. Empty slots may repeat.
func Validate(cfg Config) error {
	seen := map[string]int{}
	for i, slider := range cfg.Sliders {
		id := slider.DeviceID
		if id == "" {
			continue
		}
		if prev, ok := seen[id]; ok {
			return fmt.Errorf("slider %d and slider %d control the same device", prev+1, i+1)
		}
		seen[id] = i
	}
	return nil
}

// Load reads the config, or creates a new one.
func Load() (*Store, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, fileName)
	store := &Store{path: path, cfg: defaultConfig()}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := store.writeLocked(store.cfg); err != nil {
				return nil, err
			}
			return store, nil
		}
		return nil, err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &store.cfg); err != nil {
			return nil, err
		}
	}
	normalize(&store.cfg)
	return store, nil
}

// Snapshot returns a copy and the change number.
// The number increases only when the user saves, not when the COM port is remembered.
func (s *Store) Snapshot() (Config, uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clone(s.cfg), s.gen
}

func (s *Store) Get() Config {
	cfg, _ := s.Snapshot()
	return cfg
}

// Save writes a new user configuration and increases the change number.
func (s *Store) Save(cfg Config) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	normalize(&cfg)
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg.LastPort = s.cfg.LastPort
	cfg = clone(cfg)
	if err := s.writeLocked(cfg); err != nil {
		return err
	}
	s.cfg = cfg
	s.gen++
	return nil
}

// RememberPort stores the last working COM port without reapplying the sliders.
func (s *Store) RememberPort(port string) error {
	if port == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.LastPort == port {
		return nil
	}
	next := clone(s.cfg)
	next.LastPort = port
	if err := s.writeLocked(next); err != nil {
		return err
	}
	s.cfg = next
	return nil
}

func (s *Store) writeLocked(cfg Config) error {
	normalize(&cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func clone(cfg Config) Config {
	out := cfg
	out.Sliders = append([]Slider(nil), cfg.Sliders...)
	return out
}
