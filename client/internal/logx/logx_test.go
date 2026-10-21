package logx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetEnabled(t *testing.T) {
	dir := t.TempDir()
	Init(dir)
	t.Cleanup(func() {
		SetEnabled(true)
		closeFile()
	})

	SetEnabled(false)
	Printf("hidden %s", "nope")
	SetEnabled(true)
	Printf("visible %s", "yes")

	data, err := os.ReadFile(filepath.Join(dir, "soundmixer.log"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "hidden") {
		t.Fatalf("disabled logging wrote: %s", text)
	}
	if !strings.Contains(text, "visible yes") {
		t.Fatalf("enabled logging missing: %s", text)
	}
}

func TestFileCappedAtFiveMB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "soundmixer.log")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("START-MARKER\n"); err != nil {
		t.Fatal(err)
	}
	block := bytes.Repeat([]byte("line\n"), 8192)
	written := int64(len("START-MARKER\n"))
	for written < maxLogBytes+1024 {
		n, err := f.Write(block)
		written += int64(n)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.WriteString("END-MARKER\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	SetEnabled(true)
	t.Cleanup(closeFile)
	Init(dir)
	Printf("fresh line")

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > maxLogBytes {
		t.Fatalf("size %d, maximum %d", info.Size(), maxLogBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("START-MARKER")) {
		t.Fatal("start marker still present")
	}
	if !bytes.Contains(data, []byte("END-MARKER")) || !bytes.Contains(data, []byte("fresh line")) {
		t.Fatalf("new entries missing: %s", tail(data))
	}
	if !bytes.Contains(data, []byte("log truncated")) {
		t.Fatal("truncation notice missing")
	}
}

func tail(data []byte) string {
	if len(data) > 200 {
		data = data[len(data)-200:]
	}
	return string(data)
}
