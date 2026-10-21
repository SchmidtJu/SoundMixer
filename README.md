# SoundMixer

A USB volume mixer for Windows. Four slide potentiometers on an [ESP32-S3-Zero](https://www.waveshare.com/wiki/ESP32-S3-Zero) set the master volume of playback devices (speakers, headphones, HDMI, and so on). A small tray app applies those positions. It does not change the volume of individual programs.

The app runs on Windows 10 and Windows 11. The board talks to the PC over the same USB-C cable used for programming. There is no separate USB-serial chip and no Wi-Fi.

![SoundMixer with four sliders in its printed enclosure](docs/df667655-0f79-4571-8b5d-21c101c766cc.jpg)

```text
slide pots  →  ESP32-S3-Zero  →  USB serial  →  tray app  →  Windows playback devices
```

## Hardware

### Parts

| Qty | Part | Notes |
| --- | --- | --- |
| 1 | [ESP32-S3-Zero](https://amzn.eu/d/0aSRMNqI) | ESP32-S3FH4R2, 4 MB flash, native USB-C. The linked board is a Heemol ESP32-S3 Mini with this pinout. |
| 4 | [Slide potentiometer, 10 kΩ linear (B10K)](https://amzn.eu/d/0g2a3Muu) | 75 mm length, 60 mm travel. The linked pack is a dual-gang B103; use one gang (three pins) per slider. Linear (B), not audio taper (A). |
| 1 | USB-C cable | Must carry data. Charge-only cables never show a COM port. |
| | Wire | One shared ground, one shared 3.3 V, one wiper wire per slider. |

A multimeter is enough to find the wiper if the pot has no pin numbers. Optional: prototype board, enclosure, knob caps.

Do not substitute a 5 V board pin for the pot supply. The ESP32-S3 ADC pins are not 5 V tolerant.

### Wiring

![Wiring of the four slide potentiometers to the ESP32-S3-Zero](docs/wiring.png)

Each slider has three pins. Find them with a meter if they are not marked:

- The two end pins read about 10 kΩ between them, and that reading stays put while you move the slider.
- The wiper is the remaining pin. Resistance from the wiper to either end changes as the slider moves.

Connect every slider the same way. Slider 1 is the leftmost control in the app. The end that should be silent goes to GND (about 0 % volume). The other end goes to 3V3 (out) (about 100 % volume).

| Slider | Wiper pin | ADC channel |
| --- | --- | --- |
| 1 | GP1 | ADC1_CH0 |
| 2 | GP2 | ADC1_CH1 |
| 3 | GP3 | ADC1_CH2 |
| 4 | GP4 | ADC1_CH3 |

All four pots share the single GND pin and the single 3V3 pin on the board. Together they draw about 1.3 mA.

If one slider works backwards, swap only the GND and 3V3 wires on that slider. Leave the wiper where it is. The app can also flip all four at once (see [Settings window](#settings-window)).

Leave these pins unconnected:

- GPIO0, the BOOT button
- GPIO19 and GPIO20, the USB data pins
- GPIO21, the onboard RGB LED

GP3 is a strapping pin. It is sampled while the chip resets, and a high level there can stop a flash from starting. Before you flash, slide control 3 all the way to the GND end.

The ADC's nominal measurement range at 11 dB attenuation extends to about 3.1 V ([Espressif documentation](https://docs.espressif.com/projects/esp-idf/en/v4.4/esp32s3/api-reference/peripherals/adc.html)). With a pot supplied from 3.3 V, the reading can therefore saturate before the slider reaches its upper stop. The firmware filters the reading, and the app maps the lowest and highest 31 counts of the 0–1023 range to 0 % and 100 %. This makes both volume limits reliably reachable, with a small flat region near each end.

### Pinout

USB-C is at the top of the front view. GP1 through GP4, GND, and 3V3 are on the left side of that view.

![ESP32-S3-Zero pinout](docs/esp32-s3-zero-pinout.jpg)

Front, left row, top to bottom. Slider 1 is GP1 through slider 4 on GP4; the wiring table above is the full mapping.

| Pin | Use |
| --- | --- |
| 5V | Leave open. Do not feed the sliders from here. |
| GND | Silent end of every slider |
| 3V3 (out) | Loud end of every slider |
| GP1–GP4 | One wiper each |
| GP5, GP6 | Free |

The two onboard buttons are BOOT (marked B, wired to GPIO0) and RESET (marked R).

### Enclosure and 3D printing

The enclosure and slider knobs shown above are included in [model/](model/):

| Files | Contents |
| --- | --- |
| [Case.stl](model/stl/Case.stl), [Top.stl](model/stl/Top.stl), [Bottom.stl](model/stl/Bottom.stl) | Enclosure parts |
| [Knob.stl](model/stl/Knob.stl) | Slider knob; four are needed |
| [usb lid.stl](model/stl/usb%20lid.stl) | USB opening cover |
| [SoundMixer.3mf](model/SoundMixer.3mf) | 3MF project |
| [SoundMixer.f3d](model/SoundMixer.f3d) | Editable Autodesk Fusion design |
| [icons/](model/icons/) | SVG artwork for the slider labels |

Check the model dimensions against your sliders and board before printing. Review the printer, material, and slicing settings in your slicer before using the 3MF project. The icons are labels; playback-device assignments are configured in the app.

## Software

### Tools

Install these on the Windows PC that will run the mixer:

- [Go 1.26](https://go.dev/dl/) or newer. `go version` should print `go1.26` or higher.
- [PlatformIO](https://platformio.org/install/cli/) (`pio` on the PATH), or the PlatformIO extension in VS Code. The extension install still provides the `pio` command inside its environment.
- This repository: `git clone https://github.com/SchmidtJu/SoundMixer.git`

The first PlatformIO build downloads the ESP32 toolchain and needs an internet connection. The first `go build` downloads the Go modules the same way.

Windows 10 and 11 already include the USB driver for this board. It shows up as a COM port with USB vendor ID `303A` (Espressif), often named “USB Serial Device” or “USB JTAG/serial debug unit”. A CH340 or CP210x driver is not used.

### Flash the firmware

1. Quit the tray app if it is already running. Right-click the icon and choose **Quit**. While the app holds the COM port, esptool reports access denied.
2. Slide control 3 to the silent (GND) end.
3. Plug the board in with a data USB-C cable.
4. Open a terminal in the `firmware` folder.

Build and find the port. The first build can take several minutes.

```text
pio run
pio device list
```

If `pio` is not on the PATH, open the PlatformIO terminal in VS Code, or use `python -m platformio` in place of `pio` with the Python environment where PlatformIO is installed.

Note the COM port of the device whose vendor ID is `303A`. The steps below call it `COM5`; use the port from the list.

`pio run -t upload` is unreliable on this board. The ESP32-S3-Zero uses native USB, and the 1200-baud “touch” that PlatformIO sends does not always enter the bootloader. The port can stay busy afterwards. Flash with esptool and a normal reset instead.

PowerShell, from the `firmware` folder:

```powershell
pio pkg exec -p tool-esptoolpy -- esptool.py `
  --chip esp32s3 --port COM5 --baud 921600 `
  --before default_reset --after hard_reset `
  write_flash --flash_mode dio --flash_freq 80m --flash_size 4MB `
  0x0 .pio/build/esp32-s3-zero/bootloader.bin `
  0x8000 .pio/build/esp32-s3-zero/partitions.bin `
  0xe000 "$env:USERPROFILE/.platformio/packages/framework-arduinoespressif32/tools/partitions/boot_app0.bin" `
  0x10000 .pio/build/esp32-s3-zero/firmware.bin
```

The `boot_app0.bin` path assumes PlatformIO's default package directory. If you configured a different directory, replace that path with the location of `framework-arduinoespressif32/tools/partitions/boot_app0.bin` in your installation. PowerShell requires each continuation backtick to be the last character on its line.

For Command Prompt, join the lines into one command, remove the continuation backticks, and replace `$env:USERPROFILE` with `%USERPROFILE%`.

If that connect attempt fails, put the chip into the bootloader by hand and flash before it leaves that mode:

1. Hold BOOT (B).
2. Tap RESET (R).
3. Release BOOT.
4. Run the same esptool command with `--before no_reset` in place of `--before default_reset`.

### Check the serial link

With the tray app still closed:

```text
pio device monitor
```

Exit the monitor with Ctrl+C. Two kinds of lines are normal:

- `gpio 1=2048 2=10 3=4000 4=100` about twice a second. These are raw ADC counts from 0 to 4095.
- `1023|640|0|210` about once a second, and again whenever a slider moves. These are the four slider values, 0 to 1023, in slider order (`slider1|slider2|slider3|slider4`). This is the line the app reads.

The firmware filters each ADC reading before it sends that line. The app maps 0–1023 onto the master volume of the assigned playback endpoint.

Move each slider from end to end. The matching number should travel from near 0 to near 1023. A value stuck at either extreme usually means that wiper is tied to GND or 3V3, or is not connected.

### Build the Windows app

The app builds only on Windows. From the `client` folder:

```text
go build -ldflags "-H windowsgui -s -w" -o soundmixer.exe ./cmd/soundmixer
```

`-H windowsgui` starts the program without a console window. `client/cmd/soundmixer/rsrc.syso` is the Windows manifest and has to stay next to `main.go`. Without it, the window controls do not come up.

Start it:

```text
.\soundmixer.exe
```

The icon appears in the notification area (the system tray). If you do not see it, open the overflow arrow next to the clock. A second start does nothing useful: Windows shows **SoundMixer is already running.** and that process exits. Quit the running copy from the tray menu before starting another build.

For a console while you are changing the code:

```text
go run ./cmd/soundmixer
```

## Use

### Settings window

Left-click the tray icon, or choose **Settings…** from the menu. **Quit** closes the app.

![SoundMixer settings with four playback-device assignments](docs/Screenshot%202026-10-21%20163016.png)

Assign each slider in the settings window, then click **Save**:

- Pick a playback device, **Unassigned**, or **Windows default device**.
- The percentage beside each slider shows its current position live, including the **Invert sliders** setting. It shows a dash while disconnected. Keyboard volume changes can make the Windows volume differ from the slider position.
- **Unassigned** means that slider does nothing.
- One device can belong to only one slider. Saving a duplicate shows a warning and keeps the previous settings.
- **Windows default device** follows whatever Windows currently uses as the default playback device.
- An unplugged device stays in the list with **(disconnected)** and works again when it comes back.
- **Invert sliders** swaps the quiet and loud ends of all four sliders.
- **Start with Windows** starts the app when you sign in. After you move the exe, open settings and save once so the startup path is updated.
- **Enable logging** writes a log file. The path and the size limit are in [Configuration file](#configuration-file).
- **Volume update interval (ms)** sets the minimum time between Windows volume updates, from 20 to 2000 ms. The default is 100 ms. Lower values make slider movements respond faster. Saving applies the interval immediately.
- The status line shows **Connected (COM5)** or **Disconnected – looking for ESP32…**.

While a slider sits still, the keyboard volume keys and the Windows volume slider keep working. The next movement of that hardware slider sets the volume again.

### Configuration file

Settings are stored in `%AppData%\SoundMixer\config.json`. The folder is created the first time the app runs. A new file looks like this:

```json
{
  "com_port": "auto",
  "last_port": "",
  "invert_sliders": false,
  "autostart": false,
  "logging": true,
  "update_interval_ms": 100,
  "sliders": [
    { "device_id": "", "device_name": "" },
    { "device_id": "", "device_name": "" },
    { "device_id": "", "device_name": "" },
    { "device_id": "", "device_name": "" }
  ]
}
```

`com_port` defaults to `auto`, which searches for USB devices with vendor ID `303A`. If several match, the app prefers `last_port` when it is among them; otherwise it selects the first match. The app remembers the working port in `last_port` and also uses it as a fallback when the vendor search finds nothing, provided that port is still listed.

Set `com_port` to a name such as `COM5` to prefer that port, for example when another Espressif board is plugged in. This is a preference, not a strict binding: if the specified port is absent, the app continues with automatic discovery.

`device_id` is the Windows endpoint id. Leave both fields empty for an unassigned slider. `system.default` is the Windows default device. You can edit the file while the app is closed. With the app open, use the settings window so the running mixer picks up the change.

**Start with Windows** adds or removes a `SoundMixer` value under `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`. It points at the exe you are running. This is a per-user startup entry, not a Windows service, because audio belongs to the signed-in session.

Logging is on unless you clear **Enable logging**. The file is `%AppData%\SoundMixer\soundmixer.log` and stays within 5 MB; older lines are dropped. With the box cleared, the app stops adding lines. The file is the place to look when the tray build has no console.

## Troubleshooting

| What you see | What to do |
| --- | --- |
| No COM port | Use a data USB-C cable. Try another port. In Device Manager, look for a device with VID `303A`. |
| esptool: access denied, or the port is busy | Quit the tray app (**Quit**) and close `pio device monitor`. |
| esptool cannot connect | Slider 3 at the GND end. Then the manual BOOT + RESET sequence and `--before no_reset`. |
| Monitor prints `gpio` lines but the numbers never change | Wiper not on GP1–GP4, or a bad joint. The end-to-end resistance of the pot should stay near 10 kΩ. |
| A number stays near 0 or near 1023 | That end is tied to the wrong rail, or the wiper is shorted to an end pin. |
| Tray icon missing | Check the notification overflow (^). Confirm `rsrc.syso` was beside `main.go` when you built. |
| “SoundMixer is already running.” | The app is already in the tray. Quit it with **Quit**, then start the new exe. |
| Status stays “Disconnected – looking for ESP32…” | Check that the board is plugged in with a data cable and the firmware is flashed. Verify the COM port with `pio device list`; if `com_port` names an existing but unrelated device, set it back to `auto`. |
| “COMx is busy or not ready” | Another program has the port (serial monitor, a second copy, a flashing tool). |
| Slider moves, Windows volume does not | That slider is still **Unassigned**, or it is assigned to a device that is **(disconnected)**. |
| Top or bottom of the travel feels flat | Expected. A small dead zone at each end makes 0 % and 100 % reliable. |
| Volume jumps the wrong way | Swap GND and 3V3 on that one slider, or enable **Invert sliders** for all four. |

## Repository

| Path | Contents |
| --- | --- |
| `firmware/` | PlatformIO project. Firmware source is `firmware/src/main.cpp`. |
| `client/` | Go module. Entry point is `client/cmd/soundmixer`. |
| `model/` | Enclosure STL files, 3MF project, editable Fusion design, and SVG labels. |
| `docs/` | Project photos, app screenshot, and hardware diagrams. |
| `docs/wiring.png` | Wiring diagram used above. |
| `docs/esp32-s3-zero-pinout.jpg` | Pinout drawing used above. |
