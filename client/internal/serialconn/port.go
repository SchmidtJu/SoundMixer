//go:build windows

package serialconn

import (
	"context"
	"strings"
	"time"

	"go.bug.st/serial/enumerator"

	"soundmixer/internal/config"
)

const espressifVID = "303A"

// FindPort sucht den ESP32 über die USB-VID 303A.
// Ein fest eingestellter COM-Port hat Vorrang. Sonst der zuletzt funktionierende Port.
func FindPort(cfg config.Config) (string, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return "", err
	}

	if manual := strings.TrimSpace(cfg.ComPort); manual != "" && !strings.EqualFold(manual, "auto") {
		if portListed(ports, manual) {
			return matchName(ports, manual), nil
		}
	}

	var esp []string
	for _, port := range ports {
		if port == nil {
			continue
		}
		if strings.EqualFold(strings.TrimPrefix(port.VID, "0x"), espressifVID) {
			esp = append(esp, port.Name)
		}
	}
	if len(esp) == 0 {
		if cfg.LastPort != "" && portListed(ports, cfg.LastPort) {
			return matchName(ports, cfg.LastPort), nil
		}
		return "", errNotFound
	}
	for _, name := range esp {
		if strings.EqualFold(name, cfg.LastPort) {
			return name, nil
		}
	}
	return esp[0], nil
}

func portListed(ports []*enumerator.PortDetails, name string) bool {
	return matchName(ports, name) != ""
}

func matchName(ports []*enumerator.PortDetails, name string) string {
	for _, port := range ports {
		if port != nil && strings.EqualFold(port.Name, name) {
			return port.Name
		}
	}
	return ""
}

// RetryPause ist die Pause zwischen Verbindungsversuchen.
const RetryPause = time.Second

type notFoundError struct{}

func (notFoundError) Error() string { return "kein ESP32 gefunden" }

var errNotFound = notFoundError{}

// Sleep wartet, bricht aber ab, wenn ctx fertig ist.
func Sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
