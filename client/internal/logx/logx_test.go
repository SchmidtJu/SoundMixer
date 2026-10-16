package logx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetEnabled(t *testing.T) {
	dir := t.TempDir()
	Init(dir)
	t.Cleanup(func() { SetEnabled(true) })

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
		t.Fatalf("ausgeschaltetes logging hat geschrieben: %s", text)
	}
	if !strings.Contains(text, "visible yes") {
		t.Fatalf("eingeschaltetes logging fehlt: %s", text)
	}
}
