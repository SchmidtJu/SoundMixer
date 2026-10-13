//go:build windows

package ui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"soundmixer/internal/audio"
	"soundmixer/internal/autostart"
	"soundmixer/internal/config"
	"soundmixer/internal/logx"
	"soundmixer/internal/serialconn"
)

// App ist das Tray-Programm. Das Hauptfenster bleibt unsichtbar.
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
		link:  serialconn.LinkStatus{Detail: "Getrennt – suche ESP32…"},
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

	// Der Einstellungsdialog zeigt beim Schließen das unsichtbare Hauptfenster.
	// Wegklicken würde sonst die ganze Tray-App beenden.
	mw.Closing().Attach(func(canceled *bool, _ walk.CloseReason) {
		if !a.quitting {
			*canceled = true
			mw.SetVisible(false)
		}
	})

	settingsAction := walk.NewAction()
	if err := settingsAction.SetText("Einstellungen…"); err != nil {
		return err
	}
	settingsAction.Triggered().Attach(func() { a.openSettings() })
	if err := ni.ContextMenu().Actions().Add(settingsAction); err != nil {
		return err
	}

	exitAction := walk.NewAction()
	if err := exitAction.SetText("Beenden"); err != nil {
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

func showSettings(owner walk.Form, store *config.Store, ctrl *audio.Controller, link serialconn.LinkStatus) {
	cfg := store.Get()
	devices, err := ctrl.Devices()
	statusText := link.Label()
	if err != nil {
		logx.Printf("ausgabegeräte: %v", err)
		statusText += " – Geräteliste nicht verfügbar"
	} else {
		statusText += fmt.Sprintf(" – %d Ausgabegeräte", len(devices))
	}
	labels, ids := deviceOptions(devices, cfg)

	var dlg *walk.Dialog
	var invert *walk.CheckBox
	var auto *walk.CheckBox
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
		CheckBox{AssignTo: &invert, Text: "Slider invertieren", Checked: cfg.InvertSliders},
		CheckBox{AssignTo: &auto, Text: "Mit Windows starten", Checked: cfg.Autostart},
		Composite{
			Layout: HBox{},
			Children: []Widget{
				HSpacer{},
				PushButton{
					Text: "Speichern",
					OnClicked: func() {
						if saveSettings(dlg, store, cfg, boxes, ids, labels, invert, auto) {
							dlg.Accept()
						}
					},
				},
				PushButton{Text: "Abbrechen", OnClicked: func() { dlg.Cancel() }},
			},
		},
	)

	if err := (Dialog{
		AssignTo: &dlg,
		Title:    "SoundMixer Einstellungen",
		MinSize:  Size{Width: 480, Height: 280},
		Layout:   VBox{},
		Children: children,
	}).Create(owner); err != nil {
		logx.Printf("einstellungen: %v", err)
		walk.MsgBox(owner, "SoundMixer", "Einstellungen konnten nicht geöffnet werden.", walk.MsgBoxIconError)
		return
	}
	for i := range boxes {
		if boxes[i] == nil {
			continue
		}
		_ = boxes[i].SetCurrentIndex(indexOf(ids, sliderID(cfg, i)))
	}
	dlg.Run()
	owner.SetVisible(false)
}

func saveSettings(
	owner walk.Form,
	store *config.Store,
	previous config.Config,
	boxes []*walk.ComboBox,
	ids, labels []string,
	invert, auto *walk.CheckBox,
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
			name = strings.TrimSuffix(labels[idx], " (nicht verbunden)")
		}
		if ids[idx] == "" {
			name = ""
		}
		next.Sliders[i] = config.Slider{DeviceID: ids[idx], DeviceName: name}
	}
	next.InvertSliders = invert != nil && invert.Checked()
	next.Autostart = auto != nil && auto.Checked()

	if err := config.Validate(next); err != nil {
		walk.MsgBox(owner, "SoundMixer", err.Error(), walk.MsgBoxIconWarning)
		return false
	}
	if err := autostart.Set(next.Autostart); err != nil {
		logx.Printf("autostart: %v", err)
		walk.MsgBox(owner, "SoundMixer", "Autostart konnte nicht gesetzt werden: "+err.Error(), walk.MsgBoxIconError)
		return false
	}
	if err := store.Save(next); err != nil {
		logx.Printf("speichern: %v", err)
		_ = autostart.Set(previous.Autostart)
		walk.MsgBox(owner, "SoundMixer", "Einstellungen konnten nicht gespeichert werden.", walk.MsgBoxIconError)
		return false
	}
	return true
}

func deviceOptions(devices []audio.Device, cfg config.Config) (labels, ids []string) {
	labels = []string{"Nicht zugewiesen", "Windows-Standardgerät"}
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
		labels = append(labels, name+" (nicht verbunden)")
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
