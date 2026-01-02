package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net"
	"strings"

	"golang.org/x/crypto/curve25519"
)

func generateKeyPair() (privateKey, publicKey string, err error) {
	var privKey [32]byte
	if _, err = rand.Read(privKey[:]); err != nil {
		return "", "", fmt.Errorf("generate private key: %w", err)
	}
	privKey[0] &= 248
	privKey[31] &= 127
	privKey[31] |= 64

	var pubKey [32]byte
	curve25519.ScalarBaseMult(&pubKey, &privKey)
	return base64.StdEncoding.EncodeToString(privKey[:]),
		base64.StdEncoding.EncodeToString(pubKey[:]), nil
}

func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

// allocateIPTx allocates the next available IP using the provided CIDR.
func allocateIPTx(tx *sql.Tx, cidr string) (string, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("parse cidr: %w", err)
	}
	baseIP := ip.Mask(ipNet.Mask)
	if baseIP.To4() == nil {
		return "", fmt.Errorf("unsupported ip family")
	}

	usedIPs := make(map[string]bool)
	usedIPs[ip.String()] = true

	rows, err := tx.Query("SELECT allowed_ip FROM peers")
	if err == nil && rows != nil {
		defer rows.Close()
		for rows.Next() {
			var allowedIP string
			if err := rows.Scan(&allowedIP); err != nil {
				continue
			}
			peerIP := strings.TrimSuffix(allowedIP, "/32")
			usedIPs[peerIP] = true
		}
	}

	for i := 2; i < 255; i++ {
		nextIP := make(net.IP, 4)
		copy(nextIP, baseIP.To4())
		nextIP[3] = byte(i)

		if !usedIPs[nextIP.String()] {
			return nextIP.String(), nil
		}
	}

	return "", fmt.Errorf("no available ips")
}
