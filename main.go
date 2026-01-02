package main

import (
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	// Dispatch subcommands. Many subcommands require `loadConfig()` to read
	// `config.json` from the data dir; we call it per-command to keep startup
	// behavior explicit and make the control flow easy to follow.
	switch cmd {
	case "init":
		cmdInit()
	case "up":
		cmdUp()
	case "down":
		cmdDown()
	case "add":
		if len(os.Args) < 3 {
			fatal("Usage: vpn add <peer-name>")
		}
		cmdAddPeer(os.Args[2])
	case "remove", "rm":
		if len(os.Args) < 3 {
			fatal("Usage: vpn remove <peer-name>")
		}
		cmdRemovePeer(os.Args[2])
	case "list", "ls":
		cmdListPeers()
	case "sync":
		cmdSync()
	case "web":
		port := "8080"
		if len(os.Args) > 2 {
			port = os.Args[2]
		}
		cmdWeb(port)
	default:
		printUsage()
		os.Exit(1)
	}
}
