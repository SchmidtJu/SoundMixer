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

// Slider ordnet einen Regler einem Windows-Ausgabegerät zu.
// DeviceID ist die stabile Endpoint-ID. Leer heißt nicht zugewiesen.
// DefaultDeviceID folgt dem aktuellen Windows-Standardgerät.
type Slider struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

// Config liegt unter %AppData%\SoundMixer\config.json.
type Config struct {
	ComPort       string   `json:"com_port"`
	LastPort      string   `json:"last_port"`
	InvertSliders bool     `json:"invert_sliders"`
	Autostart     bool     `json:"autostart"`
	Logging       bool     `json:"logging"`
	Sliders       []Slider `json:"sliders"`
}

// Store hält die Konfiguration im Speicher und auf der Platte.
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

// Validate lehnt doppelte Gerätezuordnungen ab. Leere Slots dürfen mehrfach vorkommen.
func Validate(cfg Config) error {
	seen := map[string]int{}
	for i, slider := range cfg.Sliders {
		id := slider.DeviceID
		if id == "" {
			continue
		}
		if prev, ok := seen[id]; ok {
			return fmt.Errorf("Slider %d und Slider %d steuern dasselbe Gerät", prev+1, i+1)
		}
		seen[id] = i
	}
	return nil
}

// Load liest die Config oder legt eine neue an.
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
			if err := store.writeLocked(); err != nil {
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

// Snapshot liefert eine Kopie und die Änderungsnummer.
// Die Nummer steigt nur, wenn der Nutzer speichert, nicht beim Merken des COM-Ports.
func (s *Store) Snapshot() (Config, uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clone(s.cfg), s.gen
}

func (s *Store) Get() Config {
	cfg, _ := s.Snapshot()
	return cfg
}

// Save schreibt eine neue Nutzerkonfiguration und erhöht die Änderungsnummer.
func (s *Store) Save(cfg Config) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	normalize(&cfg)
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg.LastPort = s.cfg.LastPort
	s.cfg = cfg
	s.gen++
	return s.writeLocked()
}

// RememberPort merkt sich den zuletzt funktionierenden COM-Port, ohne die Regler neu anzuwenden.
func (s *Store) RememberPort(port string) error {
	if port == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.LastPort == port {
		return nil
	}
	s.cfg.LastPort = port
	return s.writeLocked()
}

func (s *Store) writeLocked() error {
	normalize(&s.cfg)
	data, err := json.MarshalIndent(s.cfg, "", "  ")
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
