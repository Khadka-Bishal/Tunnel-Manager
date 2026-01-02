package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func newManagerOrDie() *Manager {
	cfg, err := LoadConfig()
	if err != nil {
		fatal("Not initialized - run 'vpn init' first")
	}

	store, err := NewStore(cfg.DataDir)
	if err != nil {
		fatal("Failed to open database: " + err.Error())
	}

	return NewManager(cfg, store)
}

func printUsage() {
	fmt.Println("vpn - Simple WireGuard VPN Controller")
	fmt.Println()
	fmt.Println("Usage: vpn <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init          Initialize VPN server (generate keys, create config)")
	fmt.Println("  up            Bring up WireGuard interface (requires sudo)")
	fmt.Println("  down          Bring down WireGuard interface (requires sudo)")
	fmt.Println("  add <name>    Add a new peer")
	fmt.Println("  remove <name> Remove a peer")
	fmt.Println("  list          List all peers")
	fmt.Println("  sync          Sync peers to running interface (requires sudo)")
	fmt.Println("  web [port]    Start REST API (default port 8080, localhost only)")
}

func cmdInit() {
	dir := dataDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		fatal("Failed to create data dir: " + err.Error())
	}

	configPath := filepath.Join(dir, "config.json")
	if _, err := os.Stat(configPath); err == nil {
		fatal("Already initialized. Config exists at: " + configPath)
	}

	privKey, pubKey, err := generateKeyPair()
	if err != nil {
		fatal(err.Error())
	}

	cfg := &Config{
		Interface:    "wg0",
		ListenPort:   51820,
		Address:      "10.0.0.1/24",
		Endpoint:     "",
		PrivateKey:   privKey,
		PublicKey:    pubKey,
		DNS:          "1.1.1.1",
		DataDir:      dir,
		NATInterface: "eth0",
	}

	fmt.Print("Enter public endpoint (e.g., vpn.example.com or IP): ")
	var endpoint string
	fmt.Scanln(&endpoint)
	if endpoint != "" {
		cfg.Endpoint = fmt.Sprintf("%s:%d", endpoint, cfg.ListenPort)
	}

	if err := SaveConfig(cfg); err != nil {
		fatal("Failed to save config: " + err.Error())
	}

	store, err := NewStore(cfg.DataDir)
	if err != nil {
		fatal("Failed to create database: " + err.Error())
	}
	defer store.Close()

	fmt.Println("\nVPN initialized.")
	fmt.Printf("  Config: %s\n", configPath)
	fmt.Printf("  Server Public Key: %s\n", pubKey)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Run 'vpn up' to start the VPN")
	fmt.Println("  2. Run 'vpn add <name>' to add peers")
}

func cmdUp() {
	mgr := newManagerOrDie()
	defer mgr.Close()

	wgConfig, err := mgr.ServerConfig()
	if err != nil {
		fatal("Failed to build server config: " + err.Error())
	}

	wgPath := filepath.Join(mgr.cfg.DataDir, mgr.cfg.Interface+".conf")
	if err := os.WriteFile(wgPath, []byte(wgConfig), 0600); err != nil {
		fatal("Failed to write WireGuard config: " + err.Error())
	}

	if err := runSudo("wg-quick", "up", wgPath); err != nil {
		fatal("Failed to bring up interface: " + err.Error())
	}

	fmt.Println("VPN is up")
}

func cmdDown() {
	cfg, err := LoadConfig()
	if err != nil {
		fatal("Not initialized - run 'vpn init' first")
	}

	wgPath := filepath.Join(cfg.DataDir, cfg.Interface+".conf")
	if err := runSudo("wg-quick", "down", wgPath); err != nil {
		fmt.Println("Warning: " + err.Error())
	}
	fmt.Println("VPN is down")
}

func cmdAddPeer(name string) {
	mgr := newManagerOrDie()
	defer mgr.Close()

	peer, err := mgr.AddPeer(name)
	if err != nil {
		if errors.Is(err, errPeerExists) {
			fatal("Peer already exists: " + name)
		}
		fatal("Failed to save peer: " + err.Error())
	}

	fmt.Printf("Added peer: %s\n", name)
	fmt.Printf("  IP: %s\n", peer.AllowedIP)
	fmt.Println("\nClient config:")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println(mgr.ClientConfig(peer))
	fmt.Println("\nRun 'vpn sync' to apply changes to running VPN.")
}

func cmdRemovePeer(name string) {
	mgr := newManagerOrDie()
	defer mgr.Close()

	if err := mgr.RemovePeer(name); err != nil {
		if errors.Is(err, errPeerNotFound) {
			fatal("Peer not found: " + name)
		}
		fatal("Failed to delete peer: " + err.Error())
	}

	fmt.Printf("Removed peer: %s\n", name)
	fmt.Println("Run 'vpn sync' to apply changes to running VPN.")
}

func cmdListPeers() {
	mgr := newManagerOrDie()
	defer mgr.Close()

	peers, err := mgr.ListPeers()
	if err != nil {
		fatal("Failed to list peers: " + err.Error())
	}

	fmt.Printf("%-20s %-15s %-10s %s\n", "NAME", "IP", "STATUS", "CREATED")
	fmt.Println(strings.Repeat("-", 60))

	for _, peer := range peers {
		status := "enabled"
		if !peer.Enabled {
			status = "disabled"
		}
		fmt.Printf("%-20s %-15s %-10s %s\n", peer.Name, strings.TrimSuffix(peer.AllowedIP, "/32"), status, peer.CreatedAt.Format("2006-01-02"))
	}
}

func cmdSync() {
	mgr := newManagerOrDie()
	defer mgr.Close()

	wgConfig, err := mgr.ServerConfig()
	if err != nil {
		fatal("Failed to build server config: " + err.Error())
	}

	wgPath := filepath.Join(mgr.cfg.DataDir, mgr.cfg.Interface+".conf")
	if err := os.WriteFile(wgPath, []byte(wgConfig), 0600); err != nil {
		fatal("Failed to write config: " + err.Error())
	}

	peerConf := extractPeerConfig(wgConfig)
	tmpPath := filepath.Join(mgr.cfg.DataDir, "peers.conf")
	if err := os.WriteFile(tmpPath, []byte(peerConf), 0600); err != nil {
		fatal("Failed to write peers temp file: " + err.Error())
	}

	if mgr.cfg.Interface == "" {
		fatal("WireGuard interface not set in config.")
	}

	if err := runSudo("wg", "syncconf", mgr.cfg.Interface, tmpPath); err != nil {
		fatal("Failed to sync: " + err.Error())
	}
	fmt.Println("Synced peers to WireGuard")
}
func cmdWeb(port string) {
	mgr := newManagerOrDie()
	defer mgr.Close()

	api := NewAPIServer(mgr)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/peers", api.HandlePeers)
	mux.HandleFunc("/api/peer/add", api.HandleAddPeer)
	mux.HandleFunc("/api/peer/remove", api.HandleRemovePeer)
	mux.HandleFunc("/", api.NotFound)

	fmt.Printf("REST API running at http://localhost:%s\n", port)
	fmt.Println("API is bound to localhost; use SSH tunneling for remote access.")
	fmt.Println("Press Ctrl+C to stop")

	if err := http.ListenAndServe("127.0.0.1:"+port, mux); err != nil {
		fatal("Failed to start web server: " + err.Error())
	}
}
