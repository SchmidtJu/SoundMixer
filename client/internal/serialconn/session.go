//go:build windows

package serialconn

import (
	"context"
	"strings"
	"time"

	"go.bug.st/serial"

	"soundmixer/internal/audio"
	"soundmixer/internal/config"
	"soundmixer/internal/logx"
	"soundmixer/internal/proto"
	"soundmixer/internal/scale"
)

// LinkStatus describes the USB link for the tray icon and the settings window.
type LinkStatus struct {
	Connected bool
	Port      string
	Detail    string
}

func (s LinkStatus) Label() string {
	if s.Connected {
		if s.Port != "" {
			return "Connected (" + s.Port + ")"
		}
		return "Connected"
	}
	if s.Detail != "" {
		return s.Detail
	}
	return "Disconnected – looking for ESP32…"
}

func (s LinkStatus) Tooltip() string {
	return "SoundMixer – " + s.Label()
}

// Run reads slider lines and forwards changed levels to the audio controller.
func Run(ctx context.Context, store *config.Store, ctrl *audio.Controller, status func(LinkStatus)) {
	var applied [config.SliderCount]float32
	var have [config.SliderCount]bool
	var gen uint64
	force := true

	for ctx.Err() == nil {
		cfg, g := store.Snapshot()
		if g != gen {
			gen = g
			force = true
			have = [config.SliderCount]bool{}
		}

		portName, err := FindPort(cfg)
		if err != nil {
			if status != nil {
				status(LinkStatus{Detail: "Disconnected – looking for ESP32…"})
			}
			Sleep(ctx, RetryPause)
			continue
		}

		port, err := serial.Open(portName, &serial.Mode{BaudRate: 115200})
		if err != nil {
			logx.Printf("COM %s: %v", portName, err)
			if status != nil {
				status(LinkStatus{Detail: "Disconnected – " + portName + " is busy or not ready"})
			}
			Sleep(ctx, RetryPause)
			continue
		}

		_ = port.SetReadTimeout(500 * time.Millisecond)
		if status != nil {
			status(LinkStatus{Connected: true, Port: portName})
		}
		if err := store.RememberPort(portName); err != nil {
			logx.Printf("remember COM: %v", err)
		}
		logx.Printf("connected to %s", portName)

		readPort(ctx, port, store, ctrl, &gen, &force, &applied, &have)
		_ = port.Close()
		if status != nil && ctx.Err() == nil {
			status(LinkStatus{Detail: "Disconnected – looking for ESP32…"})
		}
		logx.Printf("disconnected from %s", portName)
		Sleep(ctx, 500*time.Millisecond)
	}
}

func readPort(
	ctx context.Context,
	port serial.Port,
	store *config.Store,
	ctrl *audio.Controller,
	gen *uint64,
	force *bool,
	applied *[config.SliderCount]float32,
	have *[config.SliderCount]bool,
) {
	buf := make([]byte, 256)
	var line []byte

	for ctx.Err() == nil {
		cfg, g := store.Snapshot()
		if g != *gen {
			*gen = g
			*force = true
			*have = [config.SliderCount]bool{}
		}

		n, err := port.Read(buf)
		if err != nil {
			logx.Printf("read: %v", err)
			return
		}
		if n == 0 {
			continue
		}
		line = append(line, buf[:n]...)
		for {
			idx := indexByte(line, '\n')
			if idx < 0 {
				break
			}
			raw := string(line[:idx])
			line = append([]byte(nil), line[idx+1:]...)
			if text := strings.TrimSpace(raw); strings.HasPrefix(text, "gpio ") {
				logx.Printf("%s", text)
				continue
			}
			values, err := proto.ParseLine(raw)
			if err != nil {
				continue
			}
			push(ctrl, cfg, values, force, applied, have)
		}
		if len(line) > 1024 {
			line = append([]byte(nil), line[len(line)-1024:]...)
		}
	}
}

func push(
	ctrl *audio.Controller,
	cfg config.Config,
	values [config.SliderCount]int,
	force *bool,
	applied *[config.SliderCount]float32,
	have *[config.SliderCount]bool,
) {
	for i := 0; i < config.SliderCount; i++ {
		level := scale.Level(values[i], cfg.InvertSliders)
		if !*force && have[i] && abs(level-applied[i]) < scale.Epsilon {
			continue
		}
		id := ""
		if i < len(cfg.Sliders) {
			id = cfg.Sliders[i].DeviceID
		}
		ctrl.UpdateSlider(i, id, level, id != "")
		applied[i] = level
		have[i] = true
	}
	*force = false
}

func indexByte(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
