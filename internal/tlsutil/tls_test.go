// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package tlsutil

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSelfSigned_KeyPerms(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	if _, err := LoadOrGenerateSelfSigned(certPath, keyPath, []string{"localhost"}); err != nil {
		t.Fatal(err)
	}

	certInfo, err := os.Stat(certPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := certInfo.Mode().Perm(); perm != 0o644 {
		t.Fatalf("cert perm = %o, want 0644", perm)
	}

	keyInfo, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := keyInfo.Mode().Perm(); perm != 0o600 {
		t.Fatalf("key perm = %o, want 0600", perm)
	}
}

func TestGenerateSelfSigned_SANs(t *testing.T) {
	dir := t.TempDir()
	mat, err := LoadOrGenerateSelfSigned(filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem"),
		[]string{"localhost", "127.0.0.1", "relay-edge"})
	if err != nil {
		t.Fatal(err)
	}

	block, _ := pem.Decode(mat.CertPEM)
	if block == nil {
		t.Fatal("failed to decode cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	wantDNS := map[string]bool{"localhost": true, "relay-edge": true}
	if len(cert.DNSNames) != len(wantDNS) {
		t.Fatalf("DNSNames = %v, want %v", cert.DNSNames, wantDNS)
	}
	for _, n := range cert.DNSNames {
		if !wantDNS[n] {
			t.Fatalf("unexpected DNS name %q in %v", n, cert.DNSNames)
		}
	}
	if len(cert.IPAddresses) != 1 || cert.IPAddresses[0].String() != "127.0.0.1" {
		t.Fatalf("IPAddresses = %v, want [127.0.0.1]", cert.IPAddresses)
	}
}

func TestLoadExisting_ReusesCert(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	first, err := LoadOrGenerateSelfSigned(certPath, keyPath, []string{"localhost"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrGenerateSelfSigned(certPath, keyPath, []string{"localhost"})
	if err != nil {
		t.Fatal(err)
	}
	if string(first.CertPEM) != string(second.CertPEM) {
		t.Fatal("second call regenerated the cert instead of reusing the existing one")
	}
	if string(first.KeyPEM) != string(second.KeyPEM) {
		t.Fatal("second call regenerated the key instead of reusing the existing one")
	}
}

func TestGenerate_NoSANs_DefaultsLocalhost(t *testing.T) {
	dir := t.TempDir()
	mat, err := LoadOrGenerateSelfSigned(filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem"), nil)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(mat.CertPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.DNSNames) != 1 || cert.DNSNames[0] != "localhost" {
		t.Fatalf("DNSNames = %v, want [localhost]", cert.DNSNames)
	}
}

func TestNewServer_InvalidKeypair(t *testing.T) {
	mat := Material{CertPEM: []byte("not a cert"), KeyPEM: []byte("not a key")}
	if _, err := NewServer(":0", mat, http.NotFoundHandler()); err == nil {
		t.Fatal("want error for mismatched/invalid cert-key pair, got nil")
	}
}
