# rktool

Multi-user wrapper around `rkdeveloptool`.
It provides safe shared access to embedded development boards (Rock 5B, Rock 5B+, Orange Pi 5) for multiple users on the same machine.

This setup includes:

- Serial access,
- Power management (Reboot, On, Off),
- Firmware flashing (Maskrom mode),

For several rockchip development boards.

<table style="max-width: 900px;">
  <tr>
    <td valign="middle"><a href="./pictures/setup1.jpg"><img src="./pictures/setup1.jpg"></a></td>
    <td valign="middle"><a href="./pictures/setup2.jpg"><img src="./pictures/setup2.jpg"></a></td>
  </tr>
</table>


> [!NOTE]
> Collection of scripts and tools provided as-is as a starting point. Not plug-and-play.
  
3D case: [Rock 5B 5-node cluster](https://www.printables.com/model/330063-rock-5b-5-node-cluster).

## Dependencies

- Requires the opencca manager node (hardware assembly)
- Requires a patched `rkdeveloptool` that presents a more granular `DEVLOCATION` for board selection ([`multi-manager-rkdeveloptool`](https://github.com/opencca-multi-manager/multi-manager-rkdeveloptool))
- Requires a [esp32 GPIO controller](https://github.com/opencca-multi-manager/multi-manager-esp32-gpio-controller) to put RK3588 into maskrom mode.

---

## Installation

```bash
go build -o manager .
sudo make install
```

Note: This will only copy `boards.yaml` to `/etc/opencca/boards.yaml` if not already present. Edit `/etc/opencca/boards.yaml` directly for changes.

To uninstall:

```bash
sudo make uninstall
```


## Usage

```
Usage: rktool [--board NAME] <command> [args...]

Board commands:
  uart              launch minicom on the board's serial port
  power <action>    on | off | reboot | cycle  (via smartplug)
  maskrom           put board into maskrom mode
  list              show boards and their user assignments

Debug commands:
  gpio-reset                  reset the GPIO controller
  gpio-pin <pin> <high|low>   set an ESP32 GPIO pin high or low
  smartplug <on|off>          control the smartplug directly
  uhubctl <on|off>            control the USB hub directly

All other commands are forwarded to rkdeveloptool.

Flags:
  --board NAME       select board by name (default: user's assigned board)
  --log-file / -l    minicom log file path (default: minicom.txt in current directory)
  --verbose / -v     print all commands before executing them
```

Set `RKTOOL_DEBUG=1` in the environment to permanently enable verbose output.

---

## Configuration: boards.yaml

The config lives at `/etc/opencca/boards.yaml`. A full annotated example:

```yaml
# Linux group that owns hardware access. Members bypass per-user board checks.
admin_group: opencca-admin

# Serial device of the ESP32 GPIO controller (used for maskrom pin control).
maskrom_tty: /dev/ttyGPIO

# Override binary paths if needed (these are the defaults):
# binaries:
#   minicom: /usr/bin/minicom
#   uhubctl: /usr/sbin/uhubctl
#   rkdeveloptool: /usr/local/bin/rkdeveloptool

boards:
  - name: board-1
    # Board type: rock5b | rock5b+ | orangepi
    type: rock5b

    # Serial console device. Use a stable symlink (e.g. /dev/ttyBoard1) if configured.
    tty: /dev/ttyBoard1

    # rkdeveloptool LocationID to identify this board in maskrom mode.
    # Run: sudo rkdeveloptool ld (with board in maskrom) to find this value.
    dev_location: "0x01134120"

    # smartplug base URL. Used for power on/off.
    smartplug: "http://192.33.93.126"
    # smartplug_user: admin      # optional HTTP basic auth
    # smartplug_pass: secret

    # uhubctl hub location. Run: sudo uhubctl to find the right hub.
    # Omit to disable USB power cycling for this board.
    uhubctl_id: "1-1" # optional
    # uhubctl_port: "1"           # optional

    # ESP32 GPIO pin connected to this board's maskrom button.
    maskrom_pin: "22"

    # How long to keep the board powered off during reboot/cycle/maskrom (default: 2s).
    power_off_delay: 3s

    # Delay after power-on before releasing the maskrom pin
    maskrom_release_delay: 3s

    # Optional. If set, pin is held until maskrom stable for this long, instead of after maskrom_release_delay
    maskrom_stable_for: 3s

  - name: board-2
    type: rock5b+
    tty: /dev/ttyBoard2
    dev_location: "0x01134130"
    smartplug: "http://192.33.93.129"
    uhubctl_id: "1-1.2"
    maskrom_pin: "4"

# Map Linux usernames board names. Users with multiple boards must specify --board.
assignments:
  alice:
    - board-1
  bob:
    - board-2
  charlie:
    - board-1
    - board-2
```


**Access model:** The `opencca-admin` group owns all hardware devices. The `rktool` binary is a shell wrapper that calls `sudo -g opencca-admin rktool-manager`, so any user can run `rktool` without a password while the manager enforces per-user board assignments internally.

---

## Adding a New Board

1. Wire the board's power to a smartplug and USB-C OTG port to the hub.
2. Connect the board's serial console USB adapter to the hub.
3. Wire a GPIO pin from the ESP32 to the board's maskrom button.
4. Find all hardware IDs (see below).
5. Add a `ttyBoardX` udev symlink rule and reload.
6. Add the board entry to `/etc/opencca/boards.yaml`.
7. Assign users as needed.

### Adding a symlink rule for a new device

1. Find the device's `ID_PATH`:

   ```bash
   udevadm info /dev/ttyUSB3 | grep ID_PATH
   ```

2. Add a rule to `install/99-opencca.rules`:

   ```
   SUBSYSTEM=="tty", ENV{ID_PATH}=="pci-0000:02:01.0-usb-0:1.3.4.x:1.0", SYMLINK+="ttyBoardX", GROUP="opencca-admin", MODE="0660"
   ```

3. Reinstall or reload rules:
   ```bash
   sudo make install
   # or just reload:
   sudo udevadm control --reload-rules && sudo udevadm trigger
   ```

### Finding the USB hub location (`uhubctl_id`, `uhubctl_port`)

```bash
sudo uhubctl
```

Lists all controllable hubs and their ports. Find the port where the board's USB-C OTG cable is connected. To confirm, power-cycle a port and check if the board disappears:

```bash
sudo uhubctl -l 1-1 -p 2 -a off
```

---
