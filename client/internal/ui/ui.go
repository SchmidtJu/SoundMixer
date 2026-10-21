//go:build windows

package ui

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"

	"soundmixer/internal/audio"
	"soundmixer/internal/autostart"
	"soundmixer/internal/config"
	"soundmixer/internal/logx"
	"soundmixer/internal/scale"
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
	sliderValues [config.SliderCount]int
	haveValues   bool
	valuesQueued bool
	// Accessed only on the UI thread; cleared when the dialog closes.
	refreshSettings func()
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
		if a.refreshSettings != nil {
			a.refreshSettings()
		}
	})
}

// SetSliderValues receives firmware positions independently of volume writes.
func (a *App) SetSliderValues(values [config.SliderCount]int) {
	a.mu.Lock()
	unchanged := a.haveValues && a.sliderValues == values
	a.sliderValues = values
	a.haveValues = true
	mw := a.mw
	if unchanged || mw == nil || a.valuesQueued {
		a.mu.Unlock()
		return
	}
	a.valuesQueued = true
	a.mu.Unlock()
	mw.Synchronize(func() {
		a.mu.Lock()
		a.valuesQueued = false
		a.mu.Unlock()
		if a.refreshSettings != nil {
			a.refreshSettings()
		}
	})
}

func sliderPercentText(raw int, invert, known bool) string {
	if !known {
		return "—"
	}
	return fmt.Sprintf("%d %%", int(math.Round(float64(scale.Level(raw, invert))*100)))
}

func (a *App) currentLink() serialconn.LinkStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.link
}

func (a *App) Run(showSettings bool) error {
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

	if showSettings {
		mw.Starting().Attach(a.openSettings)
	}
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
	a.showSettings(owner, a.store, a.audio, a.currentLink())
}

func (a *App) showSettings(owner *walk.MainWindow, store *config.Store, ctrl *audio.Controller, link serialconn.LinkStatus) {
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
	var updateInterval *walk.NumberEdit
	var statusLabel *walk.Label
	var percentLabels [config.SliderCount]*walk.Label
	var saveButton, cancelButton *walk.PushButton
	boxes := make([]*walk.ComboBox, config.SliderCount)

	children := []Widget{
		Composite{Layout: VBox{MarginsZero: true, Spacing: 6}, Children: []Widget{
			Label{Text: "SoundMixer", Font: Font{PointSize: 26, Bold: true}, TextColor: textPrimary},
			Label{Text: "Your sound. At your fingertips.", TextColor: textMuted},
		}},
		Composite{Background: SolidColorBrush{Color: surfaceColor}, Layout: HBox{Margins: Margins{16, 12, 16, 12}}, Children: []Widget{
			Label{AssignTo: &statusLabel, Text: statusText, TextColor: textMuted},
		}},
		Label{Text: "OUTPUT CHANNELS", Font: Font{PointSize: 9, Bold: true}, TextColor: textMuted},
	}
	for i := 0; i < config.SliderCount; i++ {
		children = append(children, Composite{
			Background: SolidColorBrush{Color: surfaceColor},
			Layout:     HBox{Margins: Margins{16, 12, 16, 12}, Spacing: 16},
			Children: []Widget{
				Label{Text: fmt.Sprintf("%02d", i+1), Font: Font{PointSize: 22, Bold: true}, TextColor: accentColor, MinSize: Size{Width: 44}},
				Composite{StretchFactor: 1, Layout: VBox{MarginsZero: true, Spacing: 5}, Children: []Widget{
					ComboBox{AssignTo: &boxes[i], Model: labels, CurrentIndex: indexOf(ids, sliderID(cfg, i)), MinSize: Size{Width: 300},
						Accessibility: Accessibility{Name: fmt.Sprintf("Slider %d playback device", i+1)}},
				}},
				Composite{Layout: VBox{MarginsZero: true, Spacing: 3}, Children: []Widget{
					Label{AssignTo: &percentLabels[i], Text: "—", Font: Font{PointSize: 20, Bold: true}, TextColor: textPrimary,
						MinSize: Size{Width: 90}, TextAlignment: AlignFar,
						ToolTipText: "Current slider position. Keyboard volume changes can make the Windows volume differ."},
				}},
			},
		})
	}
	children = append(children,
		Label{Text: "PREFERENCES", Font: Font{PointSize: 9, Bold: true}, TextColor: textMuted},
		Composite{Background: SolidColorBrush{Color: surfaceColor}, Layout: VBox{Margins: Margins{16, 14, 16, 14}, Spacing: 14}, Children: []Widget{
			Composite{Layout: HBox{MarginsZero: true, Spacing: 12}, Children: []Widget{
				Composite{StretchFactor: 1, Layout: VBox{MarginsZero: true, Spacing: 4}, Children: []Widget{
					Label{Text: "Response time", TextColor: textPrimary},
					Label{Text: "Lower values make the sliders respond faster.", TextColor: textMuted, Font: Font{PointSize: 9}},
				}},
				intervalEditor(cfg.UpdateIntervalMs, &updateInterval),
				Label{Text: "ms", TextColor: textMuted},
			}},
			Composite{Layout: HBox{MarginsZero: true, Spacing: 18}, Children: []Widget{
				CheckBox{AssignTo: &invert, Text: "Invert sliders", Checked: cfg.InvertSliders},
				CheckBox{AssignTo: &auto, Text: "Start with Windows", Checked: cfg.Autostart},
				CheckBox{AssignTo: &logging, Text: "Enable logging", Checked: cfg.Logging},
			}},
		}},
		VSpacer{},
		Composite{
			Layout: HBox{MarginsZero: true, Spacing: 10},
			Children: []Widget{
				PushButton{Text: "Logs && config", MinSize: Size{Width: 120, Height: 34},
					ToolTipText: "Open the folder containing soundmixer.log and config.json.",
					OnClicked:   func() { openDataFolder(dlg) }},
				HSpacer{},
				PushButton{AssignTo: &cancelButton, Text: "Cancel", MinSize: Size{Width: 90, Height: 34}, OnClicked: func() { dlg.Cancel() }},
				PushButton{
					AssignTo: &saveButton,
					Text:     "Save changes",
					MinSize:  Size{Width: 120, Height: 34},
					OnClicked: func() {
						if saveSettings(dlg, store, cfg, boxes, ids, labels, invert, auto, logging, updateInterval, ctrl) {
							dlg.Accept()
						}
					},
				},
			},
		},
	)

	// No owner: on close, walk would otherwise show the hidden main window
	// (SWP_SHOWWINDOW) and quit the tray app.
	if err := (Dialog{
		AssignTo:      &dlg,
		Title:         "SoundMixer",
		Font:          Font{Family: "Segoe UI", PointSize: 10},
		Background:    SolidColorBrush{Color: backgroundColor},
		MinSize:       Size{Width: 640, Height: 630},
		Layout:        VBox{Margins: Margins{24, 20, 24, 20}, Spacing: 12},
		DefaultButton: &saveButton,
		CancelButton:  &cancelButton,
		Children:      children,
	}).Create(nil); err != nil {
		logx.Printf("settings: %v", err)
		walk.MsgBox(owner, "SoundMixer", "Could not open settings.", walk.MsgBoxIconError)
		return
	}
	defer dlg.Dispose()
	applyDarkTheme(dlg)
	updateInterval.SetTextColor(textPrimary)
	deviceSummary := strings.TrimPrefix(statusText, link.Label())
	a.refreshSettings = func() {
		a.mu.Lock()
		values, known, currentLink := a.sliderValues, a.haveValues, a.link
		a.mu.Unlock()
		_ = statusLabel.SetText(currentLink.Label() + deviceSummary)
		if currentLink.Connected {
			statusLabel.SetTextColor(walk.RGB(102, 218, 174))
		} else {
			statusLabel.SetTextColor(walk.RGB(240, 190, 110))
		}
		for i, label := range percentLabels {
			_ = label.SetText(sliderPercentText(values[i], invert.Checked(), known && currentLink.Connected))
		}
	}
	defer func() { a.refreshSettings = nil }()
	invert.CheckedChanged().Attach(a.refreshSettings)
	a.refreshSettings()
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

func openDataFolder(owner walk.Form) {
	dir, err := config.Dir()
	if err == nil {
		var path *uint16
		path, err = windows.UTF16PtrFromString(dir)
		if err == nil {
			err = windows.ShellExecute(windows.Handle(owner.Handle()), nil, path, nil, nil, windows.SW_SHOWNORMAL)
		}
	}
	if err != nil {
		logx.Printf("open data folder: %v", err)
		walk.MsgBox(owner, "SoundMixer", "Could not open the logs and configuration folder: "+err.Error(), walk.MsgBoxIconError)
	}
}

func intervalEditor(ms int, target **walk.NumberEdit) NumberEdit {
	return NumberEdit{
		AssignTo:   target,
		Background: SolidColorBrush{Color: surfaceColor},
		MinSize:    Size{Width: 84},
		MaxSize:    Size{Width: 100},
		MinValue:   config.MinUpdateIntervalMs,
		MaxValue:   config.MaxUpdateIntervalMs,
		// Walk's Value property accepts float64; an int silently becomes zero.
		Value:              float64(ms),
		Increment:          10,
		SpinButtonsVisible: true,
		ToolTipText:        "20–2000 ms. Lower values make the sliders respond faster. Default: 100 ms.",
	}
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
	updateInterval *walk.NumberEdit,
	ctrl *audio.Controller,
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
	if updateInterval != nil {
		next.UpdateIntervalMs = int(updateInterval.Value())
	}

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
	ctrl.SetUpdateInterval(next.UpdateIntervalMs)
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
