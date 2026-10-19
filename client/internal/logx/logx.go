package logx

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

const maxLogBytes int64 = 5 * 1024 * 1024

var (
	mu      sync.Mutex
	logger  = log.New(os.Stderr, "", log.LstdFlags)
	logFile *os.File
	enabled = true
)

// SetEnabled schaltet das Schreiben in die Logdatei ein oder aus.
func SetEnabled(on bool) {
	mu.Lock()
	enabled = on
	mu.Unlock()
}

// Init schreibt nach %AppData%\SoundMixer\soundmixer.log.
// Die Datei bleibt bei höchstens 5 MB. Darüber fallen die ältesten Einträge weg.
// Die Release-exe hat kein Konsolenfenster, die Datei bleibt die Fehlerquelle.
func Init(dir string) {
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	// Ohne O_APPEND: unter Windows lässt sich eine Append-Datei nicht kürzen.
	f, err := os.OpenFile(filepath.Join(dir, "soundmixer.log"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return
	}
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		_ = f.Close()
		return
	}
	mu.Lock()
	if logFile != nil {
		_ = logFile.Close()
	}
	logFile = f
	logger = log.New(f, "", log.LstdFlags)
	enforceSizeLocked()
	mu.Unlock()
}

func closeFile() {
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
	logger = log.New(os.Stderr, "", log.LstdFlags)
}

func Printf(format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if !enabled {
		return
	}
	logger.Printf(format, args...)
	enforceSizeLocked()
}

// enforceSizeLocked kürzt die Datei auf die neuere Hälfte, sobald sie 5 MB überschreitet.
func enforceSizeLocked() {
	if logFile == nil {
		return
	}
	info, err := logFile.Stat()
	if err != nil || info.Size() <= maxLogBytes {
		return
	}
	keep := maxLogBytes / 2
	buf := make([]byte, keep)
	n, err := logFile.ReadAt(buf, info.Size()-keep)
	if err != nil && err != io.EOF {
		_ = logFile.Truncate(0)
		_, _ = logFile.Seek(0, io.SeekEnd)
		return
	}
	buf = buf[:n]
	if i := bytes.IndexByte(buf, '\n'); i >= 0 && i+1 < len(buf) {
		buf = buf[i+1:]
	}
	if err := logFile.Truncate(0); err != nil {
		return
	}
	if _, err := logFile.Seek(0, io.SeekStart); err != nil {
		return
	}
	_, _ = logFile.Write([]byte("logdatei gekürzt, ältere einträge entfernt\n"))
	_, _ = logFile.Write(buf)
	_, _ = logFile.Seek(0, io.SeekEnd)
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
