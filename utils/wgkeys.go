package utils

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// KeyPair represents an Ed25519 key pair
type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// GenerateKeyPair generates a new Ed25519 key pair
func GenerateKeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	return &KeyPair{
		PublicKey:  pub,
		PrivateKey: priv,
	}, nil
}

// PublicKeyToBase64 converts a public key to base64 string
func PublicKeyToBase64(pubKey ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(pubKey)
}

// PublicKeyFromBase64 converts a base64 string to public key
func PublicKeyFromBase64(pubKeyB64 string) (ed25519.PublicKey, error) {
	data, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}
	return ed25519.PublicKey(data), nil
}

// Sign signs a message with the private key
func (kp *KeyPair) Sign(message []byte) []byte {
	return ed25519.Sign(kp.PrivateKey, message)
}

// Verify verifies a signature with the public key
func (kp *KeyPair) Verify(message, signature []byte) bool {
	return ed25519.Verify(kp.PublicKey, message, signature)
}

// VerifySignature verifies a signature with a public key
func VerifySignature(pubKey ed25519.PublicKey, message, signature []byte) bool {
	return ed25519.Verify(pubKey, message, signature)
}

// SignatureToBase64 converts signature to base64
func SignatureToBase64(sig []byte) string {
	return base64.StdEncoding.EncodeToString(sig)
}

// SignatureFromBase64 converts base64 signature to bytes
func SignatureFromBase64(sigB64 string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(sigB64)
}