package main

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultPubkeyURL = "https://poixe.com/security.json"

// securityDoc is the shape of security.json.
type securityDoc struct {
	Pubkey struct {
		Kid         string `json:"kid"`
		PEM         string `json:"pem"`
		Fingerprint string `json:"fingerprint"`
	} `json:"pubkey"`
}

// loadedKey holds a parsed RSA public key with its identity.
type loadedKey struct {
	Kid         string
	Key         *rsa.PublicKey
	Fingerprint string // hex SHA-256(SPKI DER), computed locally
}

// fetchPubkey downloads and parses the public key from a security.json URL.
func fetchPubkey(ctx context.Context, client *http.Client, rawURL string) (*loadedKey, error) {
	if !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("pubkey URL must be https (got %q)", rawURL)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "poixe-cli/"+version)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch pubkey: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch pubkey: HTTP %d from %s", resp.StatusCode, rawURL)
	}

	var doc securityDoc
	if err := json.NewDecoder(io.LimitReader(resp.Body, 128<<10)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse security.json: %w", err)
	}
	return parsePubkeyDoc(doc)
}

// loadPubkeyFile reads a PEM file from disk and parses it as a public key.
func loadPubkeyFile(path, kid string) (*loadedKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pubkey file: %w", err)
	}
	if kid == "" {
		return nil, fmt.Errorf("--kid is required when using --pubkey-file")
	}
	var doc securityDoc
	doc.Pubkey.Kid = kid
	doc.Pubkey.PEM = string(b)
	return parsePubkeyDoc(doc)
}

// parsePubkeyDoc validates and parses a security doc's pubkey.
func parsePubkeyDoc(doc securityDoc) (*loadedKey, error) {
	if doc.Pubkey.Kid == "" {
		return nil, fmt.Errorf("pubkey doc missing kid")
	}
	pemData := doc.Pubkey.PEM
	if strings.Contains(pemData, "REPLACE_WITH_ACTUAL") {
		return nil, fmt.Errorf("security.json contains a placeholder public key — please replace it with the real key from https://poixe.com/security.json")
	}

	blk, _ := pem.Decode([]byte(pemData))
	if blk == nil {
		return nil, fmt.Errorf("pubkey PEM is invalid")
	}
	pub, err := x509.ParsePKIXPublicKey(blk.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse pubkey: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("pubkey is not an RSA key")
	}
	if rsaPub.N.BitLen() < 2048 {
		return nil, fmt.Errorf("pubkey is too small (%d bits, need >= 2048)", rsaPub.N.BitLen())
	}

	// Compute fingerprint locally — do not trust the value in the JSON.
	sum := sha256.Sum256(blk.Bytes)
	fp := hex.EncodeToString(sum[:])

	// If the doc includes a fingerprint, verify it matches what we computed.
	if doc.Pubkey.Fingerprint != "" &&
		!strings.Contains(doc.Pubkey.Fingerprint, "REPLACE_WITH") {
		docFP := strings.ToLower(strings.ReplaceAll(doc.Pubkey.Fingerprint, ":", ""))
		if docFP != fp {
			return nil, fmt.Errorf("pubkey fingerprint mismatch: computed %s, doc says %s", fp, doc.Pubkey.Fingerprint)
		}
	}

	return &loadedKey{Kid: doc.Pubkey.Kid, Key: rsaPub, Fingerprint: fp}, nil
}

// checkFingerprint verifies that the loaded key's fingerprint starts with expected.
// expected is normalized (colons stripped, lowercase). Must be at least 16 hex chars.
func checkFingerprint(k *loadedKey, expected string) error {
	expected = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(expected), ":", ""))
	if expected == "" {
		return nil
	}
	if len(expected) < 16 {
		return fmt.Errorf("--fingerprint must be at least 16 hex characters")
	}
	if !strings.HasPrefix(k.Fingerprint, expected) {
		return fmt.Errorf("pubkey fingerprint mismatch: expected prefix %s, got %s\npossible MITM — do not encrypt", expected, k.Fingerprint)
	}
	return nil
}
