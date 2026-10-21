//go:build windows

package ui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"soundmixer/internal/audio"
	"soundmixer/internal/autostart"
	"soundmixer/internal/config"
	"soundmixer/internal/logx"
	"soundmixer/internal/serialconn"
)

// App is the tray program. The main window stays hidden.
type App struct {
	store *config.Store
	audio *audio.Controller

	mu           sync.Mutex
	mw           *walk.MainWindow
	ni           *walk.NotifyIcon
	link         serialconn.LinkStatus
	settingsOpen bool
	quitting     bool
}

func New(store *config.Store, ctrl *audio.Controller) *App {
	return &App{
		store: store,
		audio: ctrl,
		link:  serialconn.LinkStatus{Detail: "Disconnected – looking for ESP32…"},
	}
}

func (a *App) SetStatus(status serialconn.LinkStatus) {
	a.mu.Lock()
	unchanged := a.link == status
	a.link = status
	mw := a.mw
	ni := a.ni
	a.mu.Unlock()
	if unchanged || mw == nil || ni == nil {
		return
	}
	mw.Synchronize(func() {
		_ = ni.SetToolTip(status.Tooltip())
	})
}

func (a *App) currentLink() serialconn.LinkStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.link
}

func (a *App) Run() error {
	mw, err := walk.NewMainWindow()
	if err != nil {
		return err
	}
	icon, err := mixerIcon()
	if err != nil {
		return err
	}
	_ = mw.SetIcon(icon)

	ni, err := walk.NewNotifyIcon(mw)
	if err != nil {
		return err
	}
	defer ni.Dispose()
	if err := ni.SetIcon(icon); err != nil {
		return err
	}

	a.mu.Lock()
	a.mw = mw
	a.ni = ni
	link := a.link
	a.mu.Unlock()

	if err := ni.SetToolTip(link.Tooltip()); err != nil {
		return err
	}
	if err := ni.SetVisible(true); err != nil {
		return err
	}

	// The main window only runs the message loop for the tray icon.
	// Closing it, including a close started by the dialog, must not quit the app.
	hideTrayWindow(mw)
	mw.Closing().Attach(func(canceled *bool, _ walk.CloseReason) {
		if a.quitting {
			return
		}
		*canceled = true
		hideTrayWindow(mw)
	})

	settingsAction := walk.NewAction()
	if err := settingsAction.SetText("Settings…"); err != nil {
		return err
	}
	settingsAction.Triggered().Attach(func() { a.openSettings() })
	if err := ni.ContextMenu().Actions().Add(settingsAction); err != nil {
		return err
	}

	exitAction := walk.NewAction()
	if err := exitAction.SetText("Quit"); err != nil {
		return err
	}
	exitAction.Triggered().Attach(func() {
		a.quitting = true
		walk.App().Exit(0)
	})
	if err := ni.ContextMenu().Actions().Add(exitAction); err != nil {
		return err
	}

	ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			a.openSettings()
		}
	})

	mw.Run()
	return nil
}

func (a *App) openSettings() {
	a.mu.Lock()
	if a.settingsOpen {
		a.mu.Unlock()
		return
	}
	a.settingsOpen = true
	owner := a.mw
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.settingsOpen = false
		a.mu.Unlock()
	}()
	showSettings(owner, a.store, a.audio, a.currentLink())
}

func showSettings(owner *walk.MainWindow, store *config.Store, ctrl *audio.Controller, link serialconn.LinkStatus) {
	cfg := store.Get()
	devices, err := ctrl.Devices()
	statusText := link.Label()
	if err != nil {
		logx.Printf("playback devices: %v", err)
		statusText += " – device list unavailable"
	} else {
		statusText += fmt.Sprintf(" – %d playback devices", len(devices))
	}
	labels, ids := deviceOptions(devices, cfg)

	var dlg *walk.Dialog
	var invert *walk.CheckBox
	var auto *walk.CheckBox
	var logging *walk.CheckBox
	boxes := make([]*walk.ComboBox, config.SliderCount)

	children := []Widget{
		Label{Text: statusText},
	}
	grid := []Widget{}
	for i := 0; i < config.SliderCount; i++ {
		grid = append(grid,
			Label{Text: fmt.Sprintf("Slider %d", i+1)},
			ComboBox{
				AssignTo:     &boxes[i],
				Model:        labels,
				CurrentIndex: indexOf(ids, sliderID(cfg, i)),
				MinSize:      Size{Width: 280},
			},
		)
	}
	children = append(children,
		Composite{Layout: Grid{Columns: 2}, Children: grid},
		CheckBox{AssignTo: &invert, Text: "Invert sliders", Checked: cfg.InvertSliders},
		CheckBox{AssignTo: &auto, Text: "Start with Windows", Checked: cfg.Autostart},
		CheckBox{AssignTo: &logging, Text: "Enable logging", Checked: cfg.Logging},
		Composite{
			Layout: HBox{},
			Children: []Widget{
				HSpacer{},
				PushButton{
					Text: "Save",
					OnClicked: func() {
						if saveSettings(dlg, store, cfg, boxes, ids, labels, invert, auto, logging) {
							dlg.Accept()
						}
					},
				},
				PushButton{Text: "Cancel", OnClicked: func() { dlg.Cancel() }},
			},
		},
	)

	// No owner: on close, walk would otherwise show the hidden main window
	// (SWP_SHOWWINDOW) and quit the tray app.
	if err := (Dialog{
		AssignTo: &dlg,
		Title:    "SoundMixer Settings",
		MinSize:  Size{Width: 480, Height: 310},
		Layout:   VBox{},
		Children: children,
	}).Create(nil); err != nil {
		logx.Printf("settings: %v", err)
		walk.MsgBox(owner, "SoundMixer", "Could not open settings.", walk.MsgBoxIconError)
		return
	}
	for i := range boxes {
		if boxes[i] == nil {
			continue
		}
		_ = boxes[i].SetCurrentIndex(indexOf(ids, sliderID(cfg, i)))
	}
	dlg.Starting().Attach(func() {
		_ = dlg.Activate()
		win.SetForegroundWindow(dlg.Handle())
	})
	dlg.Run()
	owner.SetEnabled(true)
	hideTrayWindow(owner)
}

// hideTrayWindow keeps the main window off the taskbar and invisible.
func hideTrayWindow(mw *walk.MainWindow) {
	hwnd := mw.Handle()
	if hwnd == 0 {
		return
	}
	ex := win.GetWindowLong(hwnd, win.GWL_EXSTYLE)
	win.SetWindowLong(hwnd, win.GWL_EXSTYLE, ex|win.WS_EX_TOOLWINDOW)
	mw.SetVisible(false)
}

func saveSettings(
	owner walk.Form,
	store *config.Store,
	previous config.Config,
	boxes []*walk.ComboBox,
	ids, labels []string,
	invert, auto, logging *walk.CheckBox,
) bool {
	next := previous
	next.Sliders = make([]config.Slider, config.SliderCount)
	for i, box := range boxes {
		idx := 0
		if box != nil && box.CurrentIndex() >= 0 {
			idx = box.CurrentIndex()
		}
		if idx >= len(ids) {
			idx = 0
		}
		name := ""
		if idx < len(labels) {
			name = strings.TrimSuffix(labels[idx], " (disconnected)")
		}
		if ids[idx] == "" {
			name = ""
		}
		next.Sliders[i] = config.Slider{DeviceID: ids[idx], DeviceName: name}
	}
	next.InvertSliders = invert != nil && invert.Checked()
	next.Autostart = auto != nil && auto.Checked()
	next.Logging = logging != nil && logging.Checked()

	if err := config.Validate(next); err != nil {
		walk.MsgBox(owner, "SoundMixer", err.Error(), walk.MsgBoxIconWarning)
		return false
	}
	if err := autostart.Set(next.Autostart); err != nil {
		logx.Printf("autostart: %v", err)
		walk.MsgBox(owner, "SoundMixer", "Could not set startup: "+err.Error(), walk.MsgBoxIconError)
		return false
	}
	if err := store.Save(next); err != nil {
		logx.Printf("save: %v", err)
		_ = autostart.Set(previous.Autostart)
		walk.MsgBox(owner, "SoundMixer", "Could not save settings.", walk.MsgBoxIconError)
		return false
	}
	logx.SetEnabled(next.Logging)
	return true
}

func deviceOptions(devices []audio.Device, cfg config.Config) (labels, ids []string) {
	labels = []string{"Unassigned", "Windows default device"}
	ids = []string{"", config.DefaultDeviceID}
	seen := map[string]bool{
		"":                     true,
		config.DefaultDeviceID: true,
	}
	for _, device := range devices {
		if device.ID == "" || seen[device.ID] {
			continue
		}
		labels = append(labels, device.Name)
		ids = append(ids, device.ID)
		seen[device.ID] = true
	}
	for _, slider := range cfg.Sliders {
		if slider.DeviceID == "" || seen[slider.DeviceID] {
			continue
		}
		name := slider.DeviceName
		if name == "" {
			name = slider.DeviceID
		}
		labels = append(labels, name+" (disconnected)")
		ids = append(ids, slider.DeviceID)
		seen[slider.DeviceID] = true
	}
	return labels, ids
}

func sliderID(cfg config.Config, index int) string {
	if index < 0 || index >= len(cfg.Sliders) {
		return ""
	}
	return cfg.Sliders[index].DeviceID
}

func indexOf(ids []string, id string) int {
	for i, candidate := range ids {
		if candidate == id {
			return i
		}
	}
	return 0
}
