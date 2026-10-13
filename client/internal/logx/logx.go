package logx

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	mu     sync.Mutex
	logger = log.New(os.Stderr, "", log.LstdFlags)
)

// Init schreibt nach %AppData%\SoundMixer\soundmixer.log.
// Die Release-exe hat kein Konsolenfenster, die Datei bleibt die Fehlerquelle.
func Init(dir string) {
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "soundmixer.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	mu.Lock()
	logger = log.New(f, "", log.LstdFlags)
	mu.Unlock()
}

func Printf(format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	logger.Printf(format, args...)
}

func Error(err error) {
	if err == nil {
		return
	}
	Printf("%v", err)
}

func Errorf(format string, args ...any) {
	Printf("%s", fmt.Sprintf(format, args...))
}
