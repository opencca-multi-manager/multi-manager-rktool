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
  gpio-reset        reset the GPIO controller
  list              show boards and their user assignments

All other commands are forwarded to rkdeveloptool.

Flags:
  --board NAME      select board by name (default: user's assigned board)
  --verbose / -v    print all commands before executing them
`

var verbose bool

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fatalf("config: %v", err)
	}

	args := os.Args[1:]
	boardFlag, args := extractFlag(args, "--board", "-b")
	verbose, args = extractBoolFlag(args, "--verbose", "-v")

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
		cmdUart(cfg, board)
	case "power":
		cmdPower(cfg, board, rest)
	case "maskrom":
		cmdMaskrom(cfg, board)
	default:
		cmdRkdeveloptool(cfg, board, args)
	}
}

// extractFlag removes a named flag and its value from args.
// Supports "--flag value" and "--flag=value".
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
