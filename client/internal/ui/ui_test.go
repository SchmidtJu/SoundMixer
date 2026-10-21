//go:build windows

package ui

import (
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"soundmixer/internal/audio"
	"soundmixer/internal/config"
)

func TestSliderPercentText(t *testing.T) {
	for _, tc := range []struct {
		raw           int
		invert, known bool
		want          string
	}{
		{0, false, true, "0 %"},
		{31, false, true, "0 %"},
		{512, false, true, "50 %"},
		{992, false, true, "100 %"},
		{1023, true, true, "0 %"},
		{0, true, true, "100 %"},
		{512, false, false, "—"},
	} {
		if got := sliderPercentText(tc.raw, tc.invert, tc.known); got != tc.want {
			t.Fatalf("%+v: got %q", tc, got)
		}
	}
}

func TestSliderPositionsRetainedBeforeWindowOpens(t *testing.T) {
	a := New(nil, audio.New())
	want := [config.SliderCount]int{0, 300, 600, 1023}
	a.SetSliderValues(want)
	if !a.haveValues || a.sliderValues != want || a.valuesQueued {
		t.Fatal("positions should be available when settings first open")
	}
}

// Exercise the actual native control and Walk property initialization, which
// a successful build alone cannot check. Test executables need the same
// Common Controls manifest as the app to create Walk's controls.
func TestIntervalEditorCreatesNativeControl(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	manifest, err := filepath.Abs("../../cmd/soundmixer/soundmixer.exe.manifest")
	if err != nil {
		t.Fatal(err)
	}
	source, err := syscall.UTF16PtrFromString(manifest)
	if err != nil {
		t.Fatal(err)
	}
	ctx := win.CreateActCtx(&win.ACTCTX{Source: source})
	if ctx == win.HANDLE(^uintptr(0)) {
		t.Fatal("could not create Common Controls activation context")
	}
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	defer kernel.NewProc("ReleaseActCtx").Call(uintptr(ctx))
	cookie, ok := win.ActivateActCtx(ctx)
	if !ok {
		t.Fatal("could not activate Common Controls manifest")
	}
	defer kernel.NewProc("DeactivateActCtx").Call(0, cookie)
	for _, ms := range []int{20, 100, 500, 2000} {
		var dlg *walk.Dialog
		var edit *walk.NumberEdit
		err := (Dialog{
			AssignTo: &dlg,
			Layout:   VBox{},
			Children: []Widget{intervalEditor(ms, &edit)},
		}).Create(nil)
		if err != nil {
			t.Fatalf("interval %d: creating settings control: %v", ms, err)
		}
		got := edit.Value()
		dlg.Dispose()
		if got != float64(ms) {
			t.Fatalf("interval %d: control contains %v", ms, got)
		}
	}
}
