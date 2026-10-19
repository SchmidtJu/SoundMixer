# SoundMixer

Hardware-Lautstärkeregler für Windows-Ausgabegeräte. Ein ESP32-S3-Zero liest vier Schiebepotentiometer und schickt die Stellungen per USB an einen Tray-Client. Der Client stellt die Master-Lautstärke der Wiedergabegeräte ein (Lautsprecher, Kopfhörer, HDMI), nicht die Lautstärke einzelner Programme.

## Verdrahtung

Der ESP32-S3-Zero hat kein USB-UART-Chip. Das USB-C-Kabel ist Programmier- und Datenleitung.

Jeder Poti ist 3-polig, 10 kΩ, am besten linear (B10K). Die Zählung ist die Pin-Nummer am Schieberegler. Alle vier Regler teilen sich denselben GND- und denselben 3V3-Pin am Board (3V3-Ausgang, nicht 5V).

| Pin am Schieberegler | ESP32-S3-Zero |
| --- | --- |
| 1 | GND |
| 2 (Schleifer) | GP1 bis GP4, ein Pin pro Regler |
| 3 | 3V3 |

- Schieberegler 1: Pin 2 an GP1 (ADC1_CH0)
- Schieberegler 2: Pin 2 an GP2 (ADC1_CH1)
- Schieberegler 3: Pin 2 an GP3 (ADC1_CH2)
- Schieberegler 4: Pin 2 an GP4 (ADC1_CH3)

Pin 1 an GND und Pin 3 an 3V3 legen die Richtung fest: zum Pin-1-Ende liest der Regler 0, zum Pin-3-Ende voll. GP3 ist beim Reset ein Strapping-Pin (JTAG-Quelle). Zum Flashen den dritten Regler zum GND-Ende schieben, damit GP3 dabei low ist.

Nicht benutzen: GPIO0 (BOOT), GPIO19/20 (USB), GPIO21 (onboard-LED).

Vier 10-kΩ-Potis ziehen zusammen etwa 1,3 mA. Der ADC des ESP32-S3 reicht bei 11-dB-Dämpfung nur bis ungefähr 3,1 V, deshalb kleben die obersten Prozent eines 3,3-V-Potis etwas. Firmware und Client fangen das mit Mittelwert und einer kleinen Totzone an beiden Enden ab.

### Pinout ESP32-S3-Zero

Oberseite mit USB-C nach oben, darunter die Rückseite. GP1 bis GP4, GND und 3V3 sitzen in der linken Reihe der Oberseite.

![Pinbelegung des ESP32-S3-Zero](docs/esp32-s3-zero-pinout.jpg)

Oberseite, linke Reihe von oben nach unten. GP1 bis GP6 können UART, PWM, I2S, ADC, I2C und SPI.

| Pin | SoundMixer |
| --- | --- |
| 5V | nicht für die Regler |
| GND | Pin 1 aller Schieberegler |
| 3V3 (out) | Pin 3 aller Schieberegler |
| GP1 | Schieberegler 1 |
| GP2 | Schieberegler 2 |
| GP3 | Schieberegler 3 |
| GP4 | Schieberegler 4 |
| GP5 | frei |
| GP6 | frei |

## Firmware flashen

Zuerst den Tray-Client beenden (**Beenden** im Kontextmenü). Solange er läuft, ist der COM-Port belegt und esptool meldet `Zugriff verweigert`. Schieberegler 3 zum GND-Ende schieben: GP3 ist beim Reset ein Strapping-Pin.

PlatformIO muss installiert sein und `pio` im PATH liegen. Im Ordner `firmware` das Image bauen und den Port des Boards suchen (USB-Gerät mit VID `303A`):

```text
pio run
pio device list
```

Fehlt `pio` im PATH, dieselben Aufrufe mit `python -m platformio` statt `pio`.

`pio run -t upload` öffnet den Port kurz mit 1200 Baud. Am ESP32-S3-Zero (USB-Serial/JTAG) landet der Chip damit nicht zuverlässig im Bootloader, und der Port bleibt danach belegt. Flashen deshalb mit esptool und `--before default_reset`. `COMx` durch den Port aus `pio device list` ersetzen. In der PowerShell `$env:USERPROFILE` statt `%USERPROFILE%` schreiben:

```text
pio pkg exec -p tool-esptoolpy -- esptool.py --chip esp32s3 --port COMx --baud 921600 --before default_reset --after hard_reset write_flash --flash_mode dio --flash_freq 80m --flash_size 4MB 0x0 .pio/build/esp32-s3-zero/bootloader.bin 0x8000 .pio/build/esp32-s3-zero/partitions.bin 0xe000 %USERPROFILE%/.platformio/packages/framework-arduinoespressif32/tools/partitions/boot_app0.bin 0x10000 .pio/build/esp32-s3-zero/firmware.bin
```

Wenn `default_reset` nicht verbindet: BOOT halten, RESET kurz tippen, BOOT loslassen und denselben Aufruf mit `--before no_reset` starten, solange der Chip im Bootloader bleibt.

Zum Mitlesen, ebenfalls nur bei beendetem Client:

```text
pio device monitor
```

Eine Zeile sieht so aus: `1023|640|0|210` (Slider 1 bis 4, Werte 0 bis 1023).

## Windows-Client

Voraussetzung: Go 1.26 oder neuer, `go` im PATH. Im Ordner `client`:

```text
go build -ldflags "-H windowsgui -s -w" -o soundmixer.exe ./cmd/soundmixer
soundmixer.exe
```

`-H windowsgui` startet die exe ohne Konsolenfenster. `cmd/soundmixer/rsrc.syso` bindet das Windows-Manifest ein und muss neben `main.go` liegen, sonst startet die Oberfläche nicht. Die exe erscheint im Infobereich. Läuft schon eine Instanz, beendet sich der zweite Start sofort. Vor dem neuen Start den alten Client über das Tray-Menü beenden.

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
- **Logging aktivieren** schreibt Meldungen nach `%AppData%\SoundMixer\soundmixer.log`. Ohne Haken kommen keine neuen Zeilen dazu. Die Datei bleibt bei höchstens 5 MB, ältere Einträge fallen weg.
- Solange ein Slider stillsteht, bleiben Tasten und der Windows-Regler wirksam. Die nächste Bewegung des Sliders setzt die Lautstärke wieder.

Die Konfiguration liegt in `%AppData%\SoundMixer\config.json`. `com_port` ist `auto` (Suche nach VID `303A`). Ein fester Port wie `COM5` ist möglich. `logging` steuert die Logdatei `%AppData%\SoundMixer\soundmixer.log` und ist ohne Angabe an.

Ein zweiter Start erkennt die laufende Instanz und beendet sich.
