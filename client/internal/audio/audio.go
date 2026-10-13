//go:build windows

package audio

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"

	"soundmixer/internal/config"
	"soundmixer/internal/logx"
)

// Device ist ein aktives Windows-Wiedergabegerät.
type Device struct {
	ID   string
	Name string
}

type sliderState struct {
	deviceID string
	level    float32
	active   bool
}

type appliedState struct {
	deviceID string
	level    float32
	ok       bool
}

type listResult struct {
	devices []Device
	err     error
}

// Controller setzt die Master-Lautstärke von Wiedergabe-Endpunkten.
// Alle COM-Aufrufe laufen auf der Goroutine von Run.
type Controller struct {
	mu      sync.Mutex
	sliders [config.SliderCount]sliderState
	dirty   bool

	wake    chan struct{}
	listReq chan chan listResult
	stop    chan struct{}
	stopped sync.Once

	vols  map[string]*wca.IAudioEndpointVolume
	last  [config.SliderCount]appliedState
	logAt map[string]time.Time
}

func New() *Controller {
	return &Controller{
		wake:    make(chan struct{}, 1),
		listReq: make(chan chan listResult),
		stop:    make(chan struct{}),
		vols:    map[string]*wca.IAudioEndpointVolume{},
		logAt:   map[string]time.Time{},
	}
}

// UpdateSlider merkt sich Ziel und Pegel. active false gibt den Regler frei.
// Der Aufruf blockiert nicht und schreibt die Lautstärke nur, wenn sich etwas ändert.
func (c *Controller) UpdateSlider(index int, deviceID string, level float32, active bool) {
	if index < 0 || index >= config.SliderCount {
		return
	}
	if level < 0 {
		level = 0
	}
	if level > 1 {
		level = 1
	}
	c.mu.Lock()
	c.sliders[index] = sliderState{
		deviceID: deviceID,
		level:    level,
		active:   active && deviceID != "",
	}
	c.dirty = true
	c.mu.Unlock()
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// Devices liefert die gerade aktiven Wiedergabegeräte.
func (c *Controller) Devices() ([]Device, error) {
	resp := make(chan listResult, 1)
	select {
	case c.listReq <- resp:
	case <-time.After(2 * time.Second):
		return nil, errors.New("audio nicht bereit")
	}
	select {
	case result := <-resp:
		return result.devices, result.err
	case <-time.After(2 * time.Second):
		return nil, errors.New("audio zeitüberschreitung")
	}
}

func (c *Controller) Stop() {
	c.stopped.Do(func() { close(c.stop) })
}

func (c *Controller) Run() {
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		logx.Printf("COM-Init: %v", err)
	}
	defer ole.CoUninitialize()

	enum, err := newEnumerator()
	if err != nil {
		logx.Printf("Audio-Enumerator: %v", err)
	}
	defer func() {
		c.releaseVolumes()
		if enum != nil {
			enum.Release()
		}
	}()

	for {
		select {
		case <-c.stop:
			return
		case resp := <-c.listReq:
			enum = ensureEnumerator(enum)
			if enum == nil {
				resp <- listResult{err: errors.New("kein Audio-Enumerator")}
				continue
			}
			devices, listErr := listDevices(enum)
			resp <- listResult{devices: devices, err: listErr}
		case <-c.wake:
			enum = ensureEnumerator(enum)
			if enum == nil {
				continue
			}
			c.flush(enum)
		}
	}
}

func ensureEnumerator(enum *wca.IMMDeviceEnumerator) *wca.IMMDeviceEnumerator {
	if enum != nil {
		return enum
	}
	created, err := newEnumerator()
	if err != nil {
		logx.Printf("Audio-Enumerator: %v", err)
		return nil
	}
	return created
}

func (c *Controller) flush(enum *wca.IMMDeviceEnumerator) {
	for {
		c.mu.Lock()
		if !c.dirty {
			c.mu.Unlock()
			return
		}
		snap := c.sliders
		c.dirty = false
		c.mu.Unlock()
		c.apply(enum, snap)
	}
}

func (c *Controller) apply(enum *wca.IMMDeviceEnumerator, snap [config.SliderCount]sliderState) {
	defaultID := ""
	defaultKnown := false
	for i, slider := range snap {
		if !slider.active {
			c.last[i] = appliedState{}
			continue
		}
		target := slider.deviceID
		if target == config.DefaultDeviceID {
			if !defaultKnown {
				id, err := defaultEndpointID(enum)
				if err != nil {
					c.logLimited("default", err)
					continue
				}
				defaultID = id
				defaultKnown = true
			}
			target = defaultID
		}
		if target == "" {
			continue
		}
		if c.last[i].ok && c.last[i].deviceID == target && abs(c.last[i].level-slider.level) < 0.001 {
			continue
		}
		if err := c.setScalar(enum, target, slider.level); err != nil {
			c.logLimited(target, err)
			continue
		}
		c.last[i] = appliedState{deviceID: target, level: slider.level, ok: true}
	}
}

func (c *Controller) setScalar(enum *wca.IMMDeviceEnumerator, id string, level float32) error {
	vol, err := c.volume(enum, id)
	if err != nil {
		return err
	}
	if err := vol.SetMasterVolumeLevelScalar(level, nil); err != nil {
		c.releaseVolume(id)
		vol, err = c.volume(enum, id)
		if err != nil {
			return err
		}
		return vol.SetMasterVolumeLevelScalar(level, nil)
	}
	return nil
}

func (c *Controller) volume(enum *wca.IMMDeviceEnumerator, id string) (*wca.IAudioEndpointVolume, error) {
	if vol, ok := c.vols[id]; ok && vol != nil {
		return vol, nil
	}
	dev, err := findDevice(enum, id)
	if err != nil {
		return nil, err
	}
	defer dev.Release()

	var vol *wca.IAudioEndpointVolume
	if err := dev.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &vol); err != nil {
		return nil, err
	}
	c.vols[id] = vol
	return vol, nil
}

func (c *Controller) releaseVolume(id string) {
	if vol := c.vols[id]; vol != nil {
		vol.Release()
	}
	delete(c.vols, id)
}

func (c *Controller) releaseVolumes() {
	for id := range c.vols {
		c.releaseVolume(id)
	}
}

func (c *Controller) logLimited(key string, err error) {
	if err == nil {
		return
	}
	now := time.Now()
	if last, ok := c.logAt[key]; ok && now.Sub(last) < 5*time.Second {
		return
	}
	c.logAt[key] = now
	logx.Printf("Lautstärke %s: %v", key, err)
}

func newEnumerator() (*wca.IMMDeviceEnumerator, error) {
	var enum *wca.IMMDeviceEnumerator
	err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &enum)
	if err != nil {
		return nil, err
	}
	return enum, nil
}

func listDevices(enum *wca.IMMDeviceEnumerator) ([]Device, error) {
	if enum == nil {
		return nil, errors.New("kein Audio-Enumerator")
	}
	var collection *wca.IMMDeviceCollection
	if err := enum.EnumAudioEndpoints(wca.ERender, wca.DEVICE_STATE_ACTIVE, &collection); err != nil {
		return nil, err
	}
	defer collection.Release()

	var count uint32
	if err := collection.GetCount(&count); err != nil {
		return nil, err
	}
	devices := make([]Device, 0, count)
	for i := uint32(0); i < count; i++ {
		var dev *wca.IMMDevice
		if err := collection.Item(i, &dev); err != nil {
			continue
		}
		id, err := deviceID(dev)
		name := friendlyName(dev)
		dev.Release()
		if err != nil || id == "" {
			continue
		}
		if name == "" {
			name = id
		}
		devices = append(devices, Device{ID: id, Name: name})
	}
	sort.Slice(devices, func(i, j int) bool {
		return strings.ToLower(devices[i].Name) < strings.ToLower(devices[j].Name)
	})
	return devices, nil
}

func findDevice(enum *wca.IMMDeviceEnumerator, id string) (*wca.IMMDevice, error) {
	var collection *wca.IMMDeviceCollection
	if err := enum.EnumAudioEndpoints(wca.ERender, wca.DEVICE_STATE_ACTIVE, &collection); err != nil {
		return nil, err
	}
	defer collection.Release()

	var count uint32
	if err := collection.GetCount(&count); err != nil {
		return nil, err
	}
	for i := uint32(0); i < count; i++ {
		var dev *wca.IMMDevice
		if err := collection.Item(i, &dev); err != nil {
			continue
		}
		got, err := deviceID(dev)
		if err != nil || got != id {
			dev.Release()
			continue
		}
		return dev, nil
	}
	return nil, errors.New("gerät nicht verbunden")
}

func defaultEndpointID(enum *wca.IMMDeviceEnumerator) (string, error) {
	var dev *wca.IMMDevice
	if err := enum.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &dev); err != nil {
		return "", err
	}
	defer dev.Release()
	return deviceID(dev)
}

func deviceID(dev *wca.IMMDevice) (string, error) {
	var id string
	if err := dev.GetId(&id); err != nil {
		return "", err
	}
	return id, nil
}

func friendlyName(dev *wca.IMMDevice) string {
	var store *wca.IPropertyStore
	if err := dev.OpenPropertyStore(wca.STGM_READ, &store); err != nil {
		return ""
	}
	defer store.Release()

	var pv wca.PROPVARIANT
	if err := store.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
		return ""
	}
	return variantString(&pv)
}

func variantString(pv *wca.PROPVARIANT) string {
	if pv.VT == ole.VT_LPWSTR && pv.Val != 0 {
		// String() gibt den COM-Speicher frei.
		return pv.String()
	}
	_ = ole.VariantClear(&pv.VARIANT)
	return ""
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
