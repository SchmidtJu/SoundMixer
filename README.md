# SoundMixer

Hardware-Lautstärkeregler für Windows-Ausgabegeräte. Ein ESP32-S3-Zero liest vier Schiebepotentiometer und schickt die Stellungen per USB an einen Tray-Client. Der Client stellt die Master-Lautstärke der Wiedergabegeräte ein (Lautsprecher, Kopfhörer, HDMI), nicht die Lautstärke einzelner Programme.

## Verdrahtung

Der ESP32-S3-Zero hat kein USB-UART-Chip. Das USB-C-Kabel ist Programmier- und Datenleitung.

Jeder Poti ist 3-polig, 10 kΩ, am besten linear (B10K):

- ein Außenpin an **3V3**
- der andere Außenpin an **GND**
- der Schleifer (Mitte) an den GPIO des Sliders

- Slider 1: GPIO1 (ADC1_CH0)
- Slider 2: GPIO2 (ADC1_CH1)
- Slider 3: GPIO4 (ADC1_CH3)
- Slider 4: GPIO5 (ADC1_CH4)

Nicht benutzen: GPIO0 (BOOT), GPIO3 (Strapping), GPIO19/20 (USB), GPIO21 (onboard-LED).

Vier 10-kΩ-Potis ziehen zusammen etwa 1,3 mA. Der ADC des ESP32-S3 reicht bei 11-dB-Dämpfung nur bis ungefähr 3,1 V, deshalb kleben die obersten Prozent eines 3,3-V-Potis etwas. Firmware und Client fangen das mit Mittelwert und einer kleinen Totzone an beiden Enden ab.

## Firmware flashen

[PlatformIO](https://platformio.org/) (CLI oder VS Code / Cursor-Erweiterung):

```text
cd firmware
pio run -t upload
```

Zum Flashen BOOT gedrückt halten, RESET kurz tippen, BOOT loslassen, dann den Upload starten. Danach erscheint ein USB-Serial-Gerät (VID `303A`, meist COM-Port).

Zum Mitlesen:

```text
pio device monitor
```

Eine Zeile sieht so aus: `1023|640|0|210` (Slider 1 bis 4, Werte 0 bis 1023).

## Windows-Client

Voraussetzung: [Go 1.26](https://go.dev/dl/) oder neuer. Im Ordner `client`:

```text
go build -ldflags "-H windowsgui -s -w" -o soundmixer.exe ./cmd/soundmixer
```

`-H windowsgui` startet die exe ohne Konsolenfenster. `cmd/soundmixer/rsrc.syso` bindet das Windows-Manifest ein und muss neben `main.go` liegen, sonst startet die Oberfläche nicht. Die Datei `soundmixer.exe` irgendwo ablegen und starten. Sie erscheint im Infobereich.

Während der Entwicklung, mit Konsole:

```text
go run ./cmd/soundmixer
```

### Bedienung

- Linksklick auf das Tray-Icon oder **Einstellungen…** im Kontextmenü öffnet die Zuordnung.
- Jeder Slider bekommt ein Ausgabegerät, **Nicht zugewiesen** oder **Windows-Standardgerät**.
- Ein Gerät kann nur einem Slider gehören.
- **Slider invertieren** tauscht oben und unten.
- **Mit Windows starten** trägt die exe unter `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` ein.
- Solange ein Slider stillsteht, bleiben Tasten und der Windows-Regler wirksam. Die nächste Bewegung des Sliders setzt die Lautstärke wieder.

Die Konfiguration liegt in `%AppData%\SoundMixer\config.json`. `com_port` ist `auto` (Suche nach VID `303A`). Ein fester Port wie `COM5` ist möglich. Meldungen stehen in `%AppData%\SoundMixer\soundmixer.log`.

Ein zweiter Start erkennt die laufende Instanz und beendet sich.
