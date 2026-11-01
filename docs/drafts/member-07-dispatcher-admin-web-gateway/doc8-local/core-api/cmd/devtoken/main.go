// Command devtoken makes a local RSA key pair and signs test access tokens.
// LOCAL USE ONLY: it refuses to run when APP_ENV=production. Real tokens come
// from Member 1's /api/v1/auth/login.
//
//	go run ./cmd/devtoken -genkey .dev            # writes .dev/jwt_private.pem and .dev/jwt_public.pem
//	go run ./cmd/devtoken -key .dev/jwt_private.pem -role ADMIN -sub <user uuid>
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"swiftdrop.local/m7/core-api/internal/platform/auth"
)

func main() {
	if os.Getenv("APP_ENV") == "production" {
		fmt.Fprintln(os.Stderr, "devtoken: refused, APP_ENV=production")
		os.Exit(2)
	}
	genkey := flag.String("genkey", "", "directory to write a new dev key pair into")
	keyFile := flag.String("key", ".dev/jwt_private.pem", "private key PEM")
	role := flag.String("role", "ADMIN", "ADMIN, DISPATCHER, COURIER, MERCHANT or CUSTOMER")
	sub := flag.String("sub", "00000000-0000-4000-8000-000000000001", "user id (uuid)")
	iss := flag.String("iss", "http://localhost", "issuer, must match JWT_ISSUER")
	ttl := flag.Duration("ttl", 15*time.Minute, "lifetime (ADR-003: 15m)")
	flag.Parse()
	if err := runDevtoken(*genkey, *keyFile, *role, *sub, *iss, *ttl); err != nil {
		fmt.Fprintln(os.Stderr, "devtoken:", err)
		os.Exit(1)
	}
}

func runDevtoken(genkey, keyFile, role, sub, iss string, ttl time.Duration) error {
	if genkey != "" {
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(genkey, 0o700); err != nil {
			return err
		}
		pub, _ := x509.MarshalPKIXPublicKey(&k.PublicKey)
		if err := os.WriteFile(filepath.Join(genkey, "jwt_private.pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustPKCS8(k)}), 0o600); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(genkey, "jwt_public.pem"), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}), 0o644)
	}
	b, err := os.ReadFile(keyFile)
	if err != nil {
		return err
	}
	blk, _ := pem.Decode(b)
	if blk == nil {
		return fmt.Errorf("%s is not PEM", keyFile)
	}
	k, err := x509.ParsePKCS8PrivateKey(blk.Bytes)
	if err != nil {
		return err
	}
	now := time.Now()
	tok, err := auth.Sign(k.(*rsa.PrivateKey), auth.Claims{Iss: iss, Sub: sub, Role: role, Iat: now.Unix(), Nbf: now.Unix(), Exp: now.Add(ttl).Unix(), Jti: fmt.Sprint(now.UnixNano())})
	if err != nil {
		return err
	}
	fmt.Println(tok)
	return nil
}

func mustPKCS8(k *rsa.PrivateKey) []byte {
	b, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		panic(err)
	}
	return b
}
