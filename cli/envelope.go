package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// envelopeParams holds optional parameters bound into the ciphertext.
// Including these in the payload prevents key reuse across different targets.
type envelopeParams struct {
	Endpoint string `json:"endpoint,omitempty"`
	Region   string `json:"region,omitempty"`
}

// envelopePayload is the plaintext payload encrypted inside the envelope.
type envelopePayload struct {
	V      int            `json:"v"`
	Type   int            `json:"t"`
	Key    string         `json:"k"`
	Params envelopeParams `json:"p,omitempty"`
}

// envelopeAAD returns the AES-GCM additional authenticated data.
// Binding the kid into AAD prevents swapping envelopes between key versions.
func envelopeAAD(kid string) []byte {
	return []byte("px-egress-v2|" + kid)
}

// seal encrypts apiKey + params into an enc:v2 envelope using the given RSA public key.
//
// Output format: enc:v2:<kid>:<b64url(RSA-OAEP(aesKey))>:<b64url(nonce)>:<b64url(AES-GCM(payload))>
//
// The payload JSON is: {"v":2,"t":<typeNum>,"k":"<apiKey>","p":{...}}
// AES-256-GCM with 12-byte nonce; tag appended by cipher.NewGCM.
// RSA-OAEP with SHA-256 for both hash and MGF1 hash, no label.
func seal(pub *rsa.PublicKey, kid string, typeNum int, apiKey string, ep envelopeParams) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("empty api key")
	}

	pl := envelopePayload{V: 2, Type: typeNum, Key: apiKey, Params: ep}
	plain, err := json.Marshal(pl)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	// Generate random AES-256 key.
	aesKey := make([]byte, 32)
	if _, err := rand.Read(aesKey); err != nil {
		return "", fmt.Errorf("generate aes key: %w", err)
	}

	blk, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(blk)
	if err != nil {
		return "", fmt.Errorf("aes-gcm: %w", err)
	}

	// 12-byte nonce.
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ct := gcm.Seal(nil, nonce, plain, envelopeAAD(kid))

	// RSA-OAEP-SHA256 wrap the AES key.
	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, aesKey, nil)
	if err != nil {
		return "", fmt.Errorf("rsa wrap: %w", err)
	}

	b64 := base64.RawURLEncoding.EncodeToString
	return "enc:v2:" + kid + ":" + b64(wrapped) + ":" + b64(nonce) + ":" + b64(ct), nil
}
