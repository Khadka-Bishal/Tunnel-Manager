package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
)

type peerView struct {
	Name      string `json:"name"`
	PublicKey string `json:"publicKey"`
	IP        string `json:"ip"`
	Enabled   bool   `json:"enabled"`
	Created   string `json:"created"`
}

type APIServer struct {
	mgr *Manager
	mu  sync.Mutex
}

func NewAPIServer(mgr *Manager) *APIServer {
	return &APIServer{mgr: mgr}
}

func (a *APIServer) HandlePeers(w http.ResponseWriter, r *http.Request) {
	peers, err := a.mgr.ListPeers()
	if err != nil {
		http.Error(w, "Failed to list peers", http.StatusInternalServerError)
		return
	}

	var out []peerView
	for _, peer := range peers {
		out = append(out, peerView{
			Name:      peer.Name,
			PublicKey: peer.PublicKey,
			IP:        strings.TrimSuffix(peer.AllowedIP, "/32"),
			Enabled:   peer.Enabled,
			Created:   peer.CreatedAt.Format("2006-01-02"),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (a *APIServer) HandleAddPeer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Name required", http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	peer, err := a.mgr.AddPeer(name)
	if err != nil {
		if errors.Is(err, errPeerExists) {
			http.Error(w, "Peer already exists", http.StatusBadRequest)
			return
		}
		http.Error(w, "Failed to save peer", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"config": a.mgr.ClientConfig(peer),
	})
}

func (a *APIServer) HandleRemovePeer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Name required", http.StatusBadRequest)
		return
	}

	if err := a.mgr.RemovePeer(name); err != nil {
		if errors.Is(err, errPeerNotFound) {
			http.Error(w, "Peer not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to delete peer", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (a *APIServer) NotFound(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not found", http.StatusNotFound)
}
