//go:build windows

package serialconn

import (
	"context"
	"testing"
	"time"

	"soundmixer/internal/audio"
	"soundmixer/internal/config"
	"soundmixer/internal/scale"
)

func TestEndStopsAreForwarded(t *testing.T) {
	for _, invert := range []bool{false, true} {
		for _, pair := range [][2]int{{40, 0}, {983, 1023}} {
			cfg := config.Config{InvertSliders: invert}
			var applied [config.SliderCount]float32
			var have [config.SliderCount]bool
			force := true
			ctrl := audio.New()
			push(ctrl, cfg, [4]int{pair[0]}, &force, &applied, &have)
			push(ctrl, cfg, [4]int{pair[1]}, &force, &applied, &have)
			if want := scale.Level(pair[1], invert); applied[0] != want {
				t.Fatalf("invert=%v, raw=%v: got %v, want %v", invert, pair, applied[0], want)
			}
			if levelChanged(applied[0], applied[0]) {
				t.Fatal("identical heartbeats must not overwrite keyboard volume")
			}
		}
	}
	if levelChanged(0.5, 0.505) {
		t.Fatal("small changes away from the end stops should still be filtered")
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

func TestReadPortWatchdog(t *testing.T) {
	for _, input := range []string{"", "gpio 1=100 2=200 3=300 4=400\n", "invalid\n", "1|2|3|1024\n"} {
		t.Run(input, func(t *testing.T) {
			t.Setenv("APPDATA", t.TempDir())
			store, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			reads := 0
			port := readerFunc(func(p []byte) (int, error) {
				time.Sleep(5 * time.Millisecond)
				reads++
				return copy(p, input), nil
			})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			var gen uint64
			force := true
			var applied [config.SliderCount]float32
			var have [config.SliderCount]bool
			ready := false
			readPort(ctx, port, store, audio.New(), &gen, &force, &applied, &have,
				25*time.Millisecond, func() { ready = true })
			if ctx.Err() != nil {
				t.Fatal("watchdog did not close the silent or invalid stream")
			}
			if reads == 0 || ready {
				t.Fatalf("reads=%d, ready=%v", reads, ready)
			}
		})
	}
}

func TestValidHeartbeatsKeepConnectionAlive(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	store, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reads, ready := 0, 0
	port := readerFunc(func(p []byte) (int, error) {
		time.Sleep(5 * time.Millisecond)
		reads++
		if reads == 10 {
			cancel()
		}
		return copy(p, "100|200|300|400\n"), nil
	})
	var gen uint64
	force := true
	var applied [config.SliderCount]float32
	var have [config.SliderCount]bool
	readPort(ctx, port, store, audio.New(), &gen, &force, &applied, &have,
		25*time.Millisecond, func() { ready++ })
	if reads != 10 || ready != 1 {
		t.Fatalf("reads=%d, ready=%d", reads, ready)
	}
}
