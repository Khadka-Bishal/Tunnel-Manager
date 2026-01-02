package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all settings.
type Config struct {
	Interface    string `json:"interface"`
	ListenPort   int    `json:"listen_port"`
	Address      string `json:"address"`
	Endpoint     string `json:"endpoint"`
	PrivateKey   string `json:"private_key"`
	PublicKey    string `json:"public_key"`
	DNS          string `json:"dns"`
	DataDir      string `json:"data_dir"`
	NATInterface string `json:"nat_interface"`
}

func dataDir() string {
	if dir := os.Getenv("VPN_DATA_DIR"); dir != "" {
		return dir
	}
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		return filepath.Join("/Users", sudoUser, ".vpn")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vpn")
}

func LoadConfig() (*Config, error) {
	dir := dataDir()
	configPath := filepath.Join(dir, "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	cfg.DataDir = dir

	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	if cfg.DataDir == "" {
		cfg.DataDir = dataDir()
	}
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return fmt.Errorf("ensure data dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	configPath := filepath.Join(cfg.DataDir, "config.json")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "Error:", msg)
	os.Exit(1)
}
