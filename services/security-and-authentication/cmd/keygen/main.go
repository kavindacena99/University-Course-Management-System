package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "key generation failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	outDir := flag.String("out-dir", "./var/keys", "directory for generated key files")
	keyID := flag.String("key-id", "", "JWT key id (defaults to a UTC timestamp)")
	force := flag.Bool("force", false, "overwrite existing key files")
	flag.Parse()

	if strings.TrimSpace(*keyID) == "" {
		*keyID = "auth-" + time.Now().UTC().Format("20060102T150405Z")
	}
	if !validKeyID(*keyID) {
		return fmt.Errorf("key id may contain only letters, digits, '-', '_', and '.'")
	}
	if err := os.MkdirAll(*outDir, 0o700); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	privatePath := filepath.Join(*outDir, "jwt-private-"+*keyID+".pem")
	publicPath := filepath.Join(*outDir, "jwt-public-"+*keyID+".pem")
	if !*force {
		for _, path := range []string{privatePath, publicPath} {
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("%s already exists; use -force to overwrite", path)
			}
		}
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate Ed25519 key: %w", err)
	}

	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("marshal public key: %w", err)
	}

	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateDER,
	})
	publicPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicDER,
	})

	if err := os.WriteFile(privatePath, privatePEM, 0o600); err != nil {
		return fmt.Errorf("write private key: %w", err)
	}
	if err := os.WriteFile(publicPath, publicPEM, 0o644); err != nil {
		return fmt.Errorf("write public key: %w", err)
	}

	publicKeysJSON, err := json.Marshal(map[string]string{
		*keyID: string(publicPEM),
	})
	if err != nil {
		return fmt.Errorf("encode public key JSON: %w", err)
	}

	fmt.Printf("key_id: %s\n", *keyID)
	fmt.Printf("private_key_file: %s\n", privatePath)
	fmt.Printf("public_key_file: %s\n", publicPath)
	fmt.Printf("AUTH_JWT_KEY_ID=%s\n", *keyID)
	fmt.Printf("AUTH_JWT_PRIVATE_KEY_FILE=%s\n", filepath.ToSlash(privatePath))
	fmt.Printf("AUTH_JWT_PUBLIC_KEYS_JSON=%s\n", publicKeysJSON)

	return nil
}

func validKeyID(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}
