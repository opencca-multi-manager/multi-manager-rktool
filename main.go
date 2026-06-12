package main

import (
	"fmt"
	"os"
)

const usage = `Usage: rktool [--board NAME] <command> [args...]

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
`

var verbose bool

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fatalf("config: %v", err)
	}

	args := os.Args[1:]
	boardFlag, args := extractFlag(args, "--board", "-b")
	logFile, args := extractFlag(args, "--log-file", "-l")
	verbose, args = extractBoolFlag(args, "--verbose", "-v")
	if os.Getenv("RKTOOL_DEBUG") != "" {
		verbose = true
	}

	if len(args) == 0 {
		fmt.Print(usage)
		runPassthrough(cfg.Binaries.Rkdeveloptool)
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		cmdList(cfg)
		return
	case "help":
		fmt.Print(usage)
		runPassthrough(cfg.Binaries.Rkdeveloptool)
		return
	case "gpio-reset":
		cmdGpioReset(cfg)
		return
	case "gpio-pin":
		if len(args) < 3 {
			fatalf("usage: gpio-pin <pin> <high|low>")
		}
		if cfg.MaskromTTY == "" {
			fatalf("maskrom_tty not configured")
		}
		if err := esp32Pin(cfg.MaskromTTY, args[1], args[2] == "high"); err != nil {
			fatalf("gpio-pin: %v", err)
		}
		return
	}

	board, err := resolveBoard(cfg, boardFlag)
	if err != nil {
		fatalf("%v", err)
	}

	if err := checkAccess(cfg, board); err != nil {
		fatalf("%v", err)
	}

	cmd, rest := args[0], args[1:]

	switch cmd {
	case "uart":
		cmdUart(cfg, board, logFile)
	case "power":
		cmdPower(cfg, board, rest)
	case "maskrom":
		cmdMaskrom(cfg, board)
	case "smartplug":
		if len(rest) == 0 {
			fatalf("usage: smartplug <on|off>")
		}
		smartplugSet(board, rest[0] == "on")
	case "uhubctl":
		if len(rest) == 0 {
			fatalf("usage: uhubctl <on|off>")
		}
		uhubctlSet(cfg, board, rest[0] == "on")
	default:
		cmdRkdeveloptool(cfg, board, args)
	}
}

func extractFlag(args []string, names ...string) (string, []string) {
	for i, a := range args {
		for _, name := range names {
			if a == name && i+1 < len(args) {
				return args[i+1], append(args[:i:i], args[i+2:]...)
			}
			if len(a) > len(name)+1 && a[:len(name)+1] == name+"=" {
				return a[len(name)+1:], append(args[:i:i], args[i+1:]...)
			}
		}
	}
	return "", args
}

func extractBoolFlag(args []string, names ...string) (bool, []string) {
	for i, a := range args {
		for _, name := range names {
			if a == name {
				return true, append(args[:i:i], args[i+1:]...)
			}
		}
	}
	return false, args
}

func logf(format string, v ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, "+ "+format+"\n", v...)
	}
}

func fatalf(format string, v ...any) {
	fmt.Fprintf(os.Stderr, "rktool: "+format+"\n", v...)
	os.Exit(1)
}
