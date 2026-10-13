//go:build windows

package instance

import (
	"golang.org/x/sys/windows"
)

// Acquire hält eine Mutex für die Sitzung. false heißt, eine Instanz läuft schon.
func Acquire() bool {
	name, err := windows.UTF16PtrFromString(`Local\SoundMixer`)
	if err != nil {
		return true
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if err == windows.ERROR_ALREADY_EXISTS {
		if handle != 0 {
			_ = windows.CloseHandle(handle)
		}
		return false
	}
	// Handle absichtlich offen lassen, sonst verschwindet die Mutex.
	return true
}

func AlertAlreadyRunning() {
	title, _ := windows.UTF16PtrFromString("SoundMixer")
	text, _ := windows.UTF16PtrFromString("SoundMixer läuft bereits.")
	_, _ = windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONINFORMATION)
}
