package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"go.bug.st/serial"
)

func cmdUart(cfg *Config, b *Board) {
	bin := cfg.Binaries.Minicom
	args := []string{bin, "-w", "-t", "xterm", "-l", "-R", "UTF-8", "-D", b.TTY, "-C", "minicom.txt", b.Name}
	logf("%s", strings.Join(args, " "))
	if err := syscall.Exec(bin, args, os.Environ()); err != nil {
		fatalf("exec minicom: %v", err)
	}
}

func cmdList(cfg *Config) {
	boardUsers := make(map[string][]string)
	for user, board := range cfg.Assignments {
		boardUsers[board] = append(boardUsers[board], user)
	}

	fmt.Printf("%-20s %-12s %s\n", "NAME", "TYPE", "ASSIGNED TO")
	fmt.Printf("%-20s %-12s %s\n", "----", "----", "-----------")
	for _, b := range cfg.Boards {
		users := boardUsers[b.Name]
		assigned := strings.Join(users, ", ")
		if assigned == "" {
			assigned = "-"
		}
		fmt.Printf("%-20s %-12s %s\n", b.Name, b.Type, assigned)
	}
}

func cmdPower(cfg *Config, b *Board, args []string) {
	if len(args) == 0 {
		fatalf("usage: power <on|off|reboot|cycle>")
	}
	switch args[0] {
	case "on":
		smartplugSet(b, true)
		uhubctlSet(cfg, b, true)
	case "off":
		uhubctlSet(cfg, b, false)
		smartplugSet(b, false)
	case "reboot":
		uhubctlSet(cfg, b, false)
		smartplugSet(b, false)
		time.Sleep(2 * time.Second)
		smartplugSet(b, true)
		uhubctlSet(cfg, b, true)
	case "cycle":
		uhubctlSet(cfg, b, false)
		smartplugSet(b, false)
		time.Sleep(5 * time.Second)
		smartplugSet(b, true)
		uhubctlSet(cfg, b, true)
	default:
		fatalf("unknown power action %q (on|off|reboot|cycle)", args[0])
	}
}

func cmdMaskrom(cfg *Config, b *Board) {
	if cfg.MaskromTTY == "" {
		fatalf("maskrom_tty not configured")
	}
	if b.MaskromPin == "" {
		fatalf("board %q has no maskrom_pin configured", b.Name)
	}

	fmt.Println("powering off board...")
	uhubctlSet(cfg, b, false)
	smartplugSet(b, false)
	time.Sleep(2 * time.Second)

	fmt.Printf("asserting maskrom pin %s high via ESP32...\n", b.MaskromPin)
	if err := esp32Pin(cfg.MaskromTTY, b.MaskromPin, true); err != nil {
		fatalf("esp32: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	fmt.Println("powering on board...")
	smartplugSet(b, true)
	uhubctlSet(cfg, b, true)
	time.Sleep(500 * time.Millisecond)

	fmt.Printf("releasing maskrom pin %s via ESP32...\n", b.MaskromPin)
	if err := esp32Pin(cfg.MaskromTTY, b.MaskromPin, false); err != nil {
		fatalf("esp32: %v", err)
	}

	waitMaskrom(cfg, b)
}

func cmdGpioReset(cfg *Config) {
	if cfg.MaskromTTY == "" {
		fatalf("maskrom_tty not configured")
	}
	if err := esp32Reset(cfg.MaskromTTY); err != nil {
		fatalf("gpio-reset: %v", err)
	}
}

// esp32Reset resets the ESP32 by toggling RTS. Needed after power cycle
func esp32Reset(tty string) error {
	port, err := serial.Open(tty, &serial.Mode{BaudRate: 115200})
	if err != nil {
		return fmt.Errorf("open %s: %w", tty, err)
	}
	defer port.Close()

	if err := port.SetDTR(false); err != nil {
		return err
	}
	if err := port.SetRTS(true); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	return port.SetRTS(false)
}

// esp32Pin sends a "SET <pin> HIGH\n" or "SET <pin> LOW\n" command to an ESP32,
// then reads and checks the response.
func esp32Pin(tty, pin string, high bool) error {
	port, err := serial.Open(tty, &serial.Mode{BaudRate: 115200})
	if err != nil {
		return fmt.Errorf("open %s: %w", tty, err)
	}
	defer port.Close()

	if err = port.ResetInputBuffer(); err != nil {
		return fmt.Errorf("flush input: %w", err)
	}

	level := "LOW"
	if high {
		level = "HIGH"
	}
	if _, err = fmt.Fprintf(port, "SET %s %s\n", pin, level); err != nil {
		return err
	}

	if err = port.SetReadTimeout(500 * time.Millisecond); err != nil {
		return fmt.Errorf("set read timeout: %w", err)
	}
	var resp []byte
	buf := make([]byte, 1)
	for {
		n, err := port.Read(buf)
		if n > 0 {
			resp = append(resp, buf[:n]...)
			if buf[0] == '\n' {
				break
			}
		}
		if err != nil {
			break
		}
	}
	line := strings.TrimSpace(string(resp))
	if strings.HasPrefix(line, "ERR:") {
		return fmt.Errorf("esp32: %s", line)
	}
	if !strings.HasPrefix(line, "OK") {
		return fmt.Errorf("esp32: unexpected response: %q", line)
	}
	return nil
}

func isMaskrom(cfg *Config, b *Board) bool {
	cmd := exec.Command(cfg.Binaries.Rkdeveloptool, "ld")
	cmd.Env = rkEnv(b.DevLocation)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "Maskrom")
}

func waitMaskrom(cfg *Config, b *Board) {
	const retryInterval = 500 * time.Millisecond
	const timeout = 60 * time.Second
	start := time.Now()
	deadline := start.Add(timeout)
	for time.Now().Before(deadline) {
		elapsed := time.Since(start).Round(time.Second)
		fmt.Printf("\rwaiting for maskrom device... %s", elapsed)
		if isMaskrom(cfg, b) {
			fmt.Println("\ndevice is in maskrom mode")
			return
		}
		time.Sleep(retryInterval)
	}
	fmt.Println()
	fatalf("timed out waiting for maskrom device")
}

func rkEnv(loc string) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "DEVLOCATION=") {
			env = append(env, e)
		}
	}
	return append(env, "DEVLOCATION="+loc)
}

func runRkdeveloptool(bin string, loc string, args ...string) {
	logf("DEVLOCATION=%s %s %s", loc, bin, strings.Join(args, " "))
	cmd := exec.Command(bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = rkEnv(loc)
	if err := cmd.Run(); err != nil {
		os.Exit(exitCode(err))
	}
}

func cmdRkdeveloptool(cfg *Config, b *Board, args []string) {
	runRkdeveloptool(cfg.Binaries.Rkdeveloptool, b.DevLocation, args...)
}

func uhubctlSet(cfg *Config, b *Board, on bool) {
	if b.UhubctlID == "" {
		return
	}
	action := "off"
	if on {
		action = "on"
	}
	runSilent(cfg.Binaries.Uhubctl, "-l", b.UhubctlID, "-a", action)
}

func smartplugSet(b *Board, on bool) {
	if b.SmartPlug == "" {
		fmt.Println("no smartplug configured for board, skipping")
		return
	}
	state := "off"
	if on {
		state = "on"
	}
	url := fmt.Sprintf("%s/cm?cmnd=Power%%20%s", b.SmartPlug, state)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fatalf("smartplug %s: %v", state, err)
	}
	if b.SmartPlugUser != "" || b.SmartPlugPass != "" {
		req.SetBasicAuth(b.SmartPlugUser, b.SmartPlugPass)
	}
	resp, err := http.DefaultClient.Do(req) //nolint:noctx
	if err != nil {
		fatalf("smartplug %s: %v", state, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	fmt.Printf("smartplug: power %s\n", state)
}

func runPassthrough(path string, args ...string) {
	logf("%s %s", path, strings.Join(args, " "))
	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(exitCode(err))
	}
}

func runSilent(name string, args ...string) {
	logf("%s %s", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		fatalf("%s: %v\n%s", name, err, out)
	}
}

func exitCode(err error) int {
	if e, ok := err.(*exec.ExitError); ok {
		return e.ExitCode()
	}
	return 1
}
