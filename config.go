package main

import (
	"fmt"
	"os"
	"os/user"

	"gopkg.in/yaml.v3"
)

type BoardType string

const (
	Rock5B     BoardType = "rock5b"
	Rock5BPlus BoardType = "rock5b+"
	OrangePi   BoardType = "orangepi"
)

type Board struct {
	Name            string    `yaml:"name"`
	Type            BoardType `yaml:"type"`
	TTY             string    `yaml:"tty"`
	DevLocation     string    `yaml:"dev_location"`
	SmartPlug       string    `yaml:"smartplug"`
	SmartPlugUser   string    `yaml:"smartplug_user,omitempty"`
	SmartPlugPass   string    `yaml:"smartplug_pass,omitempty"`
	UhubctlID       string    `yaml:"uhubctl_id"`
	MaskromPin      string    `yaml:"maskrom_pin,omitempty"`
}

type Binaries struct {
	Minicom       string `yaml:"minicom"`
	Uhubctl       string `yaml:"uhubctl"`
	Rkdeveloptool string `yaml:"rkdeveloptool"`
}

type Config struct {
	AdminGroup  string            `yaml:"admin_group"`
	MaskromTTY  string            `yaml:"maskrom_tty,omitempty"`
	Binaries    Binaries          `yaml:"binaries"`
	Boards      []Board           `yaml:"boards"`
	Assignments map[string]string `yaml:"assignments"` // linux user -> board name
}

var configPaths = []string{
	"/etc/opencca/boards.yaml",
}

func loadConfig() (*Config, error) {
	var path string
	for _, p := range configPaths {
		if _, err := os.Stat(p); err == nil {
			path = p
			break
		}
	}
	if path == "" {
		return nil, fmt.Errorf("no config file found (place boards.yaml in /etc/opencca/)")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := Config{
		Binaries: Binaries{
			Minicom:       "/usr/bin/minicom",
			Uhubctl:       "/usr/sbin/uhubctl",
			Rkdeveloptool: "/usr/bin/rkdeveloptool",
		},
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &cfg, nil
}

func (cfg *Config) validate() error {
	boardNames := make(map[string]bool, len(cfg.Boards))
	for _, b := range cfg.Boards {
		boardNames[b.Name] = true
	}
	for user, board := range cfg.Assignments {
		if !boardNames[board] {
			return fmt.Errorf("assignment for user %q references unknown board %q", user, board)
		}
	}
	return nil
}

func (cfg *Config) boardByName(name string) (*Board, error) {
	for i := range cfg.Boards {
		if cfg.Boards[i].Name == name {
			return &cfg.Boards[i], nil
		}
	}
	return nil, fmt.Errorf("board %q not found", name)
}

func resolveBoard(cfg *Config, boardFlag string) (*Board, error) {
	if boardFlag != "" {
		return cfg.boardByName(boardFlag)
	}

	u, err := currentUser()
	if err == nil {
		if name, ok := cfg.Assignments[u]; ok {
			return cfg.boardByName(name)
		}
	}

	return nil, fmt.Errorf("no board assigned to current user, use --board NAME")
}

func checkAccess(cfg *Config, b *Board) error {
	u, err := currentUser()
	if err != nil {
		return fmt.Errorf("cannot determine current user: %w", err)
	}

	if isGroupMember(cfg.AdminGroup) {
		return nil
	}

	if assigned, ok := cfg.Assignments[u]; ok && assigned == b.Name {
		return nil
	}

	return fmt.Errorf("user %q is not allowed to access board %q", u, b.Name)
}

func currentUser() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

func isGroupMember(group string) bool {
	if group == "" {
		return false
	}
	g, err := user.LookupGroup(group)
	if err != nil {
		return false
	}
	u, err := user.Current()
	if err != nil {
		return false
	}
	ids, err := u.GroupIds()
	if err != nil {
		return false
	}
	for _, id := range ids {
		if id == g.Gid {
			return true
		}
	}
	return false
}
