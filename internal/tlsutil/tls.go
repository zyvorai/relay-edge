// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Material holds PEM-encoded cert and key.
type Material struct {
	CertPEM []byte
	KeyPEM  []byte
}

// LoadOrGenerateSelfSigned returns persisted TLS material or creates a new
// self-signed cert with the given SAN entries (DNS names and/or IPs).
func LoadOrGenerateSelfSigned(certPath, keyPath string, sans []string) (Material, error) {
	if b, err := os.ReadFile(certPath); err == nil {
		k, err2 := os.ReadFile(keyPath)
		if err2 == nil {
			return Material{CertPEM: b, KeyPEM: k}, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(certPath), 0o755); err != nil {
		return Material{}, err
	}
	mat, err := generate(sans)
	if err != nil {
		return Material{}, err
	}
	if err := os.WriteFile(certPath, mat.CertPEM, 0o644); err != nil {
		return Material{}, err
	}
	if err := os.WriteFile(keyPath, mat.KeyPEM, 0o600); err != nil {
		return Material{}, err
	}
	return mat, nil
}

func generate(sans []string) (Material, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Material{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return Material{}, err
	}
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "relay-edge"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, s := range sans {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if ip := net.ParseIP(s); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, s)
		}
	}
	if len(tmpl.DNSNames) == 0 && len(tmpl.IPAddresses) == 0 {
		tmpl.DNSNames = []string{"localhost"}
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return Material{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return Material{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return Material{CertPEM: certPEM, KeyPEM: keyPEM}, nil
}

// ListenAndServe starts an HTTPS server with the given PEM material.
func ListenAndServe(addr string, mat Material, handler http.Handler) error {
	cert, err := tls.X509KeyPair(mat.CertPEM, mat.KeyPEM)
	if err != nil {
		return fmt.Errorf("tls keypair: %w", err)
	}
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	return srv.ListenAndServeTLS("", "")
}
