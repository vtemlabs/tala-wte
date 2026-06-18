// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/pocketbase/pocketbase"

	"github.com/vtemlabs/tala-wte/internal/api"
)

var safeCertName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

const (
	pkiDir = "/var/lib/tala-wte/pki"
	caDir  = pkiDir + "/ca"

	// ServerCertName is the canonical FreeRADIUS server certificate name.
	ServerCertName = "radius-server"
)

// CADir returns the directory holding the CA and issued certificates.
func CADir() string { return caDir }

// CAExists reports whether the CA private material is present.
func CAExists() bool {
	if _, err := os.Stat(filepath.Join(caDir, "ca.crt")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(caDir, "ca.key")); err != nil {
		return false
	}
	return true
}

// CertExists reports whether a non-CA certificate with the given name exists.
func CertExists(name string) bool {
	if _, err := os.Stat(filepath.Join(caDir, name+".crt")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(caDir, name+".key")); err != nil {
		return false
	}
	return true
}

// EnsureCA initializes the CA if it does not already exist.
func EnsureCA() error {
	if CAExists() {
		return nil
	}
	return initCA()
}

// EnsureServerCert generates a server certificate for `name` if missing.
func EnsureServerCert(name string) error {
	if !safeCertName.MatchString(name) {
		return fmt.Errorf("invalid certificate name: only alphanumeric, dash, underscore allowed")
	}
	if CertExists(name) {
		return nil
	}
	return generateCert(name, false)
}

// CreateCAHandler initializes a new CA.
func CreateCAHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := initCA(); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, map[string]any{"status": "ca_created", "pki_dir": caDir})
	}
}

// CreateServerCertHandler generates a server certificate signed by the CA.
func CreateServerCertHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		name := r.URL.Query().Get("name")
		if name == "" {
			name = "radius-server"
		}
		if !safeCertName.MatchString(name) {
			api.WriteErr(w, http.StatusBadRequest, "invalid certificate name: only alphanumeric, dash, and underscore allowed")
			return
		}
		if err := generateCert(name, false); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, map[string]any{"status": "server_cert_created", "name": name})
	}
}

// CreateClientCertHandler generates a client certificate for EAP-TLS.
func CreateClientCertHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		uid := r.URL.Query().Get("uid")
		if uid == "" {
			api.WriteErr(w, http.StatusBadRequest, "uid required")
			return
		}
		if !safeCertName.MatchString(uid) {
			api.WriteErr(w, http.StatusBadRequest, "invalid uid: only alphanumeric, dash, and underscore allowed")
			return
		}
		if err := generateCert(uid+"-client", true); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, map[string]any{"status": "client_cert_created", "uid": uid})
	}
}

func initCA() error {
	if err := os.MkdirAll(caDir, 0o750); err != nil {
		return fmt.Errorf("mkdir pki: %w", err)
	}

	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	caTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Tala WTE"},
			Country:      []string{"US"},
			CommonName:   "Tala WTE CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	return writeMaterial("ca", caBytes, priv)
}

func generateCert(name string, isClient bool) error {
	caCertData, err := os.ReadFile(filepath.Join(caDir, "ca.crt"))
	if err != nil {
		return err
	}
	caBlock, _ := pem.Decode(caCertData)
	if caBlock == nil {
		return fmt.Errorf("failed to decode CA certificate PEM")
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return err
	}

	caKeyData, err := os.ReadFile(filepath.Join(caDir, "ca.key"))
	if err != nil {
		return err
	}
	caKeyBlock, _ := pem.Decode(caKeyData)
	if caKeyBlock == nil {
		return fmt.Errorf("failed to decode CA private key PEM")
	}
	caPrivKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return err
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Tala WTE"},
			Country:      []string{"US"},
			CommonName:   name,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(1, 0, 0),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		DNSNames:  []string{name},
	}

	if isClient {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, caCert, &priv.PublicKey, caPrivKey)
	if err != nil {
		return err
	}

	return writeMaterial(name, certBytes, priv)
}

func writeMaterial(name string, certBytes []byte, priv *rsa.PrivateKey) error {
	certOut, err := os.Create(filepath.Join(caDir, name+".crt"))
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		return err
	}

	keyOut, err := os.OpenFile(filepath.Join(caDir, name+".key"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	privBytes := x509.MarshalPKCS1PrivateKey(priv)
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}); err != nil {
		return err
	}
	return nil
}

// EnsureServerCerts guarantees that a CA and Server SSL Material exist.
func EnsureServerCerts(hostname string) (crtPath string, keyPath string, err error) {
	if _, err := os.Stat(caDir); os.IsNotExist(err) {
		if err := initCA(); err != nil {
			return "", "", fmt.Errorf("failed creating internal CA: %w", err)
		}
	}
	crtPath = filepath.Join(caDir, hostname+".crt")
	keyPath = filepath.Join(caDir, hostname+".key")

	if _, err := os.Stat(crtPath); os.IsNotExist(err) {
		if err := generateCert(hostname, false); err != nil {
			return "", "", fmt.Errorf("failed generating server cert %s: %w", hostname, err)
		}
	}
	return crtPath, keyPath, nil
}
