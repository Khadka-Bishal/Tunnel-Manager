package main

import (
	"errors"
	"time"
)

type Peer struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	PublicKey  string    `json:"public_key"`
	PrivateKey string    `json:"private_key,omitempty"`
	AllowedIP  string    `json:"allowed_ip"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
}

var (
	errPeerExists   = errors.New("peer already exists")
	errPeerNotFound = errors.New("peer not found")
)
