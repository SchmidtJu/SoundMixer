---
name: ESP32 Soundmixer
overview: "Ein USB-Soundmixer wie deej: Der ESP32-S3-Zero liest vier Schiebepotis und schickt die Werte per USB-Serial. Ein schlanker Go-Tray-Client stellt damit die Lautstärke der Windows-Ausgabegeräte ein; die Zuordnung Slider zu Gerät erfolgt in einem Einstellungsfenster."
todos:
  - id: firmware
    content: "PlatformIO-Firmware: 4 ADC-Pins, Mittelwert, USB-CDC, Zeilenprotokoll 0-1023"
    status: completed
  - id: audio
    content: "Go: Wiedergabe-Endpunkte auflisten und Master-Lautstärke per Endpoint-ID setzen"
    status: completed
  - id: serial
    content: "Go: COM-Port per VID 0x303A finden, Zeilen lesen, bei Trennung neu verbinden"
    status: completed
  - id: ui
    content: Tray-Icon, Einstellungsfenster, config.json, Autostart, Einzelinstanz
    status: completed
  - id: readme
    content: README mit Verdrahtung, Flashen (BOOT+RESET) und Build der exe
    status: completed
isProject: false
---

# ESP32 Soundmixer für Windows-Ausgabegeräte

Go ist hier die richtige Wahl: eine einzelne `.exe`, kein Python-Laufzeitpaket, schneller Start, und die Windows-Audio-API ist damit stabil anbindbar. deej macht dasselbe Muster (Arduino plus Go), steuert aber vor allem Programme. Dieser Mixer steuert nur **Wiedergabe-Endgeräte** (Lautsprecher, Kopfhörer, HDMI), nicht einzelne Apps.

```mermaid
flowchart LR
  pots[Vier 10k Slider] --> esp[ESP32-S3-Zero]
  esp -->|"USB Serial 0-1023"| client[Go Tray Client]
  client --> wasapi[Windows Core Audio]
  wasapi --> devs[Ausgabegeraete]
  client --> cfg[config.json]
```

## Hardware und Verdrahtung

ESP32-S3-Zero hat **kein** USB-UART-Chip. `Serial` läuft über den nativen USB-Port (CDC). Zum Flashen: BOOT halten, RESET tippen, dann loslassen.

Jeder Poti (3-polig, 10 kΩ, idealerweise **linear / B10K**): Außenpins an **3V3** und **GND**, Schleifer an einen ADC1-Pin. Vier Potis ziehen zusammen nur etwa 1,3 mA.

Pins (nur ADC1, keine Strap-/USB-/LED-Pins):

- Slider 1: GPIO1 (ADC1_CH0)
- Slider 2: GPIO2 (ADC1_CH1)
- Slider 3: GPIO4 (ADC1_CH3)
- Slider 4: GPIO5 (ADC1_CH4)

Nicht verwenden: GPIO0 (BOOT), GPIO3 (Strapping), GPIO19/20 (USB), GPIO21 (onboard WS2812).

Der ESP32-S3-ADC reicht bei 12-dB-Dämpfung nur bis etwa 3,1 V. Die oberen Prozent des 3,3-V-Potis können deshalb kleben. Firmware mittelt mehrere Samples; der Client setzt an beiden Enden eine kleine Totzone, damit 0 % und 100 % sicher erreicht werden.

## Firmware

PlatformIO, Arduino-Framework, Board-Basis `esp32-s3-devkitc-1` (4 MB Flash, passend zum ESP32-S3FH4R2). Flags: `ARDUINO_USB_CDC_ON_BOOT=1`, `ARDUINO_USB_MODE=1`.

Protokoll, eine Zeile pro Messung, kompatibel zum deej-Stil:

```text
1023|640|0|210
```

Werte 0–1023, Slider-Reihenfolge links nach rechts. Senden nur bei Änderung (Schwelle ein paar Stufen) und höchstens etwa alle 30 ms, plus ein Herzschlag etwa einmal pro Sekunde, damit der Client Verbindungsabbruch erkennt. Pro Pin mehrere `analogRead`-Werte mitteln.

## Windows-Client (Go)

Eine `.exe` ohne Konsolenfenster (`-H windowsgui`), eine Instanz (Mutex). Liegt als Tray-Icon, kein dauerhaftes Hauptfenster.

Module:

- **Serial:** `go.bug.st/serial`. Port automatisch über USB-VID `0x303A` (Espressif) suchen, sonst der zuletzt funktionierende COM-Port aus der Config. Bei Ziehen des Kabels neu verbinden.
- **Audio:** `github.com/moutend/go-wca`. Nur `eRender`-Endpunkte. Lautstärke über `IAudioEndpointVolume.SetMasterVolumeLevelScalar` (0.0–1.0). Keine App-Sessions.
- **Zuordnung:** stabile Windows-Endpoint-ID speichern, Anzeigename nur für die Liste. Fällt ein Gerät weg (Kopfhörer ausgesteckt), bleibt die Zuordnung; der Slider greift wieder, sobald es da ist.
- **Tray und Fenster:** `github.com/lxn/walk` mit `NotifyIcon`, damit Tray und Dialog denselben UI-Thread nutzen. Eigenes Systray-Paket plus separates GUI-Toolkit würde sich auf dem Windows-Message-Loop in die Quere kommen.

Verhalten der Regler: nur schreiben, wenn sich der skalierte Wert wirklich ändert, sonst poppt die Windows-Lautstärkeanzeige dauernd auf. Solange der Slider steht, kann die Tastatur die Lautstärke weiter ändern; die nächste Slider-Bewegung überschreibt sie. Das ist das übliche Mixer-Verhalten.

Einstellungen, aus dem Tray geöffnet:

- vier Dropdowns: aktive Ausgabegeräte, plus „nicht zugewiesen“ und „Windows-Standardgerät“ (folgt dem aktuellen Standardgerät)
- ein Gerät nur einem Slider zuordenbar
- Slider invertieren
- mit Windows starten (HKCU Run, kein Dienst — Audio gehört zur Benutzersitzung)
- Status: verbunden, COM-Port oder getrennt

Config: `%AppData%\SoundMixer\config.json`.

## Projektlayout

- [firmware/](firmware/) — PlatformIO-Projekt, `src/main.cpp`
- [client/](client/) — Go-Modul, `cmd/soundmixer` plus Pakete `serialconn`, `audio`, `config`, `ui`
- [README.md](README.md) — Verdrahtung, Flash-Ablauf, Build der `.exe`

## Bewusst nicht enthalten

Keine App-Lautstärke, kein Mikrofon, kein WLAN, kein Linux. Rauschfilter ist eine feste Totzone plus Mittelwert, keine extra Stufen wie bei deej.
