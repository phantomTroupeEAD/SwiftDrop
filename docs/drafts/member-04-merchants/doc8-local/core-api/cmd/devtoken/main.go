// Command devtoken creates a local RSA key pair and signs test access tokens.
// LOCAL USE ONLY (refuses APP_ENV=production). Real tokens come from Member 1.
//
//	go run ./cmd/devtoken -genkey ../.dev
//	go run ./cmd/devtoken -key ../.dev/jwt_private.pem -role MERCHANT -sub <uuid> -tenant <merchant uuid>
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"swiftdrop.local/m4/core-api/internal/platform/auth"
)

func main() {
	if os.Getenv("APP_ENV") == "production" {
		fmt.Fprintln(os.Stderr, "devtoken: refused, APP_ENV=production")
		os.Exit(2)
	}
	genkey := flag.String("genkey", "", "directory to write a new dev key pair into")
	keyFile := flag.String("key", "../.dev/jwt_private.pem", "private key PEM")
	role := flag.String("role", "ADMIN", "ADMIN, MERCHANT, CUSTOMER, ...")
	sub := flag.String("sub", "00000000-0000-4000-8000-0000000000ad", "user id")
	tenant := flag.String("tenant", "", "tenant_id (merchant id) for MERCHANT tokens")
	iss := flag.String("iss", "http://localhost", "issuer, must match JWT_ISSUER")
	ttl := flag.Duration("ttl", 15*time.Minute, "lifetime (ADR-003: 15m)")
	flag.Parse()
	if err := run(*genkey, *keyFile, *role, *sub, *tenant, *iss, *ttl); err != nil {
		fmt.Fprintln(os.Stderr, "devtoken:", err)
		os.Exit(1)
	}
}

func run(genkey, keyFile, role, sub, tenant, iss string, ttl time.Duration) error {
	if genkey != "" {
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(genkey, 0o700); err != nil {
			return err
		}
		pub, _ := x509.MarshalPKIXPublicKey(&k.PublicKey)
		if err := os.WriteFile(filepath.Join(genkey, "jwt_private.pem"),
			pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}), 0o600); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(genkey, "jwt_public.pem"),
			pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}), 0o644)
	}
	b, err := os.ReadFile(keyFile)
	if err != nil {
		return err
	}
	blk, _ := pem.Decode(b)
	if blk == nil {
		return fmt.Errorf("%s is not PEM", keyFile)
	}
	k, err := x509.ParsePKCS1PrivateKey(blk.Bytes)
	if err != nil {
		return err
	}
	jti := make([]byte, 8)
	_, _ = rand.Read(jti)
	now := time.Now()
	tok, err := auth.Sign(k, auth.Claims{Iss: iss, Sub: sub, Role: role, TenantID: tenant,
		Exp: now.Add(ttl).Unix(), Nbf: now.Unix(), Iat: now.Unix(), Jti: "jwt_" + hex.EncodeToString(jti)})
	if err != nil {
		return err
	}
	fmt.Print(tok)
	return nil
}
