package main

type Manager struct {
	cfg   *Config
	store *Store
}

func NewManager(cfg *Config, store *Store) *Manager {
	return &Manager{cfg: cfg, store: store}
}

func (m *Manager) AddPeer(name string) (*Peer, error) {
	return m.store.CreatePeer(name, m.cfg.Address)
}

func (m *Manager) RemovePeer(name string) error {
	return m.store.RemovePeer(name)
}

func (m *Manager) ListPeers() ([]Peer, error) {
	return m.store.ListPeers()
}

func (m *Manager) EnabledPeers() ([]Peer, error) {
	return m.store.EnabledPeers()
}

func (m *Manager) ServerConfig() (string, error) {
	peers, err := m.store.EnabledPeers()
	if err != nil {
		return "", err
	}
	return generateServerConfig(m.cfg, peers), nil
}

func (m *Manager) ClientConfig(peer *Peer) string {
	return generateClientConfig(m.cfg, peer)
}

func (m *Manager) Close() error {
	return m.store.Close()
}
