//go:build windows

package instance

import (
	"golang.org/x/sys/windows"
)

// Acquire holds a session mutex. false means an instance is already running.
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
	// Leave the handle open. Closing it drops the mutex.
	return true
}

func AlertAlreadyRunning() {
	title, _ := windows.UTF16PtrFromString("SoundMixer")
	text, _ := windows.UTF16PtrFromString("SoundMixer is already running.")
	_, _ = windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONINFORMATION)
}
