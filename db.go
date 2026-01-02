package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func NewStore(dir string) (*Store, error) {
	if dir == "" {
		dir = dataDir()
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	_ = os.Chmod(dir, 0700)

	dbPath := filepath.Join(dir, "vpn.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	_ = os.Chmod(dbPath, 0600)

	if err := ensureSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func ensureSchema(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS peers (
		id TEXT PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		public_key TEXT UNIQUE NOT NULL,
		private_key TEXT,
		allowed_ip TEXT UNIQUE NOT NULL,
		enabled INTEGER DEFAULT 1,
		created_at DATETIME NOT NULL
	)`); err != nil {
		return fmt.Errorf("create peers table: %w", err)
	}

	rows, err := db.Query("PRAGMA table_info(peers)")
	if err != nil {
		return fmt.Errorf("table info: %w", err)
	}
	defer rows.Close()

	hasPrivateKey := false
	for rows.Next() {
		var cid int
		var colName, colType string
		var notnull, dfltVal, pk interface{}
		if err := rows.Scan(&cid, &colName, &colType, &notnull, &dfltVal, &pk); err != nil {
			return fmt.Errorf("scan table info: %w", err)
		}
		if colName == "private_key" {
			hasPrivateKey = true
			break
		}
	}
	if !hasPrivateKey {
		if _, err := db.Exec("ALTER TABLE peers ADD COLUMN private_key TEXT"); err != nil {
			return fmt.Errorf("migrate private_key: %w", err)
		}
	}

	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreatePeer(name, cidr string) (*Peer, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}

	var exists int
	if err := tx.QueryRow("SELECT COUNT(*) FROM peers WHERE name = ?", name).Scan(&exists); err != nil {
		tx.Rollback()
		return nil, err
	}
	if exists > 0 {
		tx.Rollback()
		return nil, errPeerExists
	}

	privKey, pubKey, err := generateKeyPair()
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	ip, err := allocateIPTx(tx, cidr)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	id, err := generateID()
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	peer := &Peer{
		ID:         id,
		Name:       name,
		PublicKey:  pubKey,
		PrivateKey: privKey,
		AllowedIP:  ip + "/32",
		Enabled:    true,
		CreatedAt:  time.Now(),
	}

	if _, err := tx.Exec(`INSERT INTO peers (id, name, public_key, private_key, allowed_ip, enabled, created_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		peer.ID, peer.Name, peer.PublicKey, peer.PrivateKey, peer.AllowedIP, peer.Enabled, peer.CreatedAt); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return nil, err
	}

	return peer, nil
}

func (s *Store) RemovePeer(name string) error {
	result, err := s.db.Exec("DELETE FROM peers WHERE name = ?", name)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errPeerNotFound
	}
	return nil
}

func (s *Store) ListPeers() ([]Peer, error) {
	rows, err := s.db.Query("SELECT id, name, public_key, private_key, allowed_ip, enabled, created_at FROM peers ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var peers []Peer
	for rows.Next() {
		var p Peer
		if err := rows.Scan(&p.ID, &p.Name, &p.PublicKey, &p.PrivateKey, &p.AllowedIP, &p.Enabled, &p.CreatedAt); err != nil {
			return nil, err
		}
		peers = append(peers, p)
	}

	return peers, rows.Err()
}

func (s *Store) EnabledPeers() ([]Peer, error) {
	rows, err := s.db.Query("SELECT public_key, allowed_ip FROM peers WHERE enabled = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var peers []Peer
	for rows.Next() {
		var p Peer
		if err := rows.Scan(&p.PublicKey, &p.AllowedIP); err != nil {
			return nil, err
		}
		peers = append(peers, p)
	}
	return peers, rows.Err()
}
