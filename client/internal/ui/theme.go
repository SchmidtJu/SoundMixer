//go:build windows

package ui

import (
	"syscall"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

var (
	commonControls     = windows.NewLazySystemDLL("comctl32.dll")
	setSubclass        = commonControls.NewProc("SetWindowSubclass")
	defSubclass        = commonControls.NewProc("DefSubclassProc")
	removeSubclass     = commonControls.NewProc("RemoveWindowSubclass")
	comboThemeCallback uintptr
)

func init() {
	comboThemeCallback = syscall.NewCallback(func(hwnd win.HWND, msg uint32, wParam, lParam, id, brush uintptr) uintptr {
		if msg == win.WM_CTLCOLORLISTBOX {
			win.SetTextColor(win.HDC(wParam), win.COLORREF(textPrimary))
			win.SetBkColor(win.HDC(wParam), win.COLORREF(surfaceColor))
			return brush
		}
		if msg == win.WM_NCDESTROY {
			removeSubclass.Call(uintptr(hwnd), comboThemeCallback, id)
		}
		result, _, _ := defSubclass.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
		return result
	})
}

var (
	backgroundColor = walk.RGB(18, 20, 27)
	surfaceColor    = walk.RGB(29, 32, 42)
	textPrimary     = walk.RGB(237, 239, 247)
	textMuted       = walk.RGB(159, 167, 188)
	accentColor     = walk.RGB(170, 153, 255)
)

// Theme native controls as well as the surrounding Walk surfaces. These calls
// are best-effort: older Windows versions retain their native control chrome.
func applyDarkTheme(dlg *walk.Dialog) {
	theme := windows.NewLazySystemDLL("uxtheme.dll").NewProc("SetWindowTheme")
	dwm := windows.NewLazySystemDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	if dwm.Find() == nil {
		enabled := int32(1)
		dwm.Call(uintptr(dlg.Handle()), 20, uintptr(unsafe.Pointer(&enabled)), unsafe.Sizeof(enabled))
	}
	if theme.Find() != nil {
		return
	}
	name, _ := syscall.UTF16PtrFromString("DarkMode_Explorer")
	comboName, _ := syscall.UTF16PtrFromString("DarkMode_CFD")
	brush := win.CreateBrushIndirect(&win.LOGBRUSH{LbStyle: win.BS_SOLID, LbColor: win.COLORREF(surfaceColor)})
	if brush != 0 {
		dlg.Disposing().Attach(func() { win.DeleteObject(win.HGDIOBJ(brush)) })
	}
	var visit func(walk.Window)
	visit = func(window walk.Window) {
		controlTheme := name
		if _, ok := window.(*walk.ComboBox); ok {
			controlTheme = comboName
			if brush != 0 {
				setSubclass.Call(uintptr(window.Handle()), comboThemeCallback, 1, uintptr(brush))
			}
		}
		theme.Call(uintptr(window.Handle()), uintptr(unsafe.Pointer(controlTheme)), 0)
		if container, ok := window.(walk.Container); ok {
			children := container.Children()
			for i := 0; i < children.Len(); i++ {
				visit(children.At(i))
			}
		}
	}
	visit(dlg)
}
