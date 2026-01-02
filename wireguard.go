package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// generateServerConfig builds the server WireGuard config from local config
// and enabled peers in the database. Keep the format compatible with wg-quick.

func generateServerConfig(cfg *Config, peers []Peer) string {
	var sb strings.Builder

	// Server interface
	sb.WriteString("[Interface]\n")
	sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", cfg.PrivateKey))
	sb.WriteString(fmt.Sprintf("Address = %s\n", cfg.Address))
	sb.WriteString(fmt.Sprintf("ListenPort = %d\n", cfg.ListenPort))

	// NAT rules - only for Linux (iptables), skip on macOS
	if cfg.NATInterface != "" && runtime.GOOS == "linux" {
		sb.WriteString(fmt.Sprintf("PostUp = iptables -A FORWARD -i %%i -j ACCEPT; iptables -t nat -A POSTROUTING -o %s -j MASQUERADE\n", cfg.NATInterface))
		sb.WriteString(fmt.Sprintf("PostDown = iptables -D FORWARD -i %%i -j ACCEPT; iptables -t nat -D POSTROUTING -o %s -j MASQUERADE\n", cfg.NATInterface))
	}

	for _, peer := range peers {
		sb.WriteString("\n[Peer]\n")
		sb.WriteString(fmt.Sprintf("PublicKey = %s\n", peer.PublicKey))
		sb.WriteString(fmt.Sprintf("AllowedIPs = %s\n", peer.AllowedIP))
	}

	return sb.String()
}

func generateClientConfig(cfg *Config, peer *Peer) string {
	var sb strings.Builder

	sb.WriteString("[Interface]\n")
	sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", peer.PrivateKey))
	sb.WriteString(fmt.Sprintf("Address = %s\n", peer.AllowedIP))
	if cfg.DNS != "" {
		sb.WriteString(fmt.Sprintf("DNS = %s\n", cfg.DNS))
	}

	sb.WriteString("\n[Peer]\n")
	sb.WriteString(fmt.Sprintf("PublicKey = %s\n", cfg.PublicKey))
	if cfg.Endpoint != "" {
		sb.WriteString(fmt.Sprintf("Endpoint = %s\n", cfg.Endpoint))
	}
	sb.WriteString("AllowedIPs = 0.0.0.0/0\n")
	sb.WriteString("PersistentKeepalive = 25\n")

	return sb.String()
}

// extractPeerConfig extracts just the [Peer] sections for wg syncconf
func extractPeerConfig(config string) string {
	lines := strings.Split(config, "\n")
	var result []string
	inPeer := false
	for _, line := range lines {
		if strings.HasPrefix(line, "[Peer]") {
			inPeer = true
		} else if strings.HasPrefix(line, "[Interface]") {
			inPeer = false
		}
		if inPeer {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func runSudo(name string, args ...string) error {
	allArgs := append([]string{name}, args...)
	cmd := exec.Command("sudo", allArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
