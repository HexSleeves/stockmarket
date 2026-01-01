package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
)

// Config holds application configuration
type Config struct {
	Port            string
	DatabasePath    string
	EncryptionKey   []byte // 32 bytes for AES-256
	Environment     string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./stockmarket.db"
	}

	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	// Encryption key - in production, this should come from a secure source
	encKeyStr := os.Getenv("ENCRYPTION_KEY")
	var encKey []byte
	if encKeyStr != "" {
		var err error
		encKey, err = base64.StdEncoding.DecodeString(encKeyStr)
		if err != nil || len(encKey) != 32 {
			return nil, errors.New("ENCRYPTION_KEY must be a base64-encoded 32-byte key")
		}
	} else {
		// Generate a random key for development (not persisted!)
		encKey = make([]byte, 32)
		if _, err := rand.Read(encKey); err != nil {
			return nil, err
		}
	}

	return &Config{
		Port:          port,
		DatabasePath:  dbPath,
		EncryptionKey: encKey,
		Environment:   env,
	}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM
func Encrypt(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func Decrypt(ciphertext string, key []byte) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
