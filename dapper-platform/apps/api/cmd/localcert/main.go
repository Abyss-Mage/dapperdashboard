// Generates local development files only. Does not install a trust certificate.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

func main() {
	directory := flag.String("directory", "", "new private certificate directory")
	flag.Parse()
	if *directory == "" {
		log.Fatal("Explicit new directory required")
	}
	if err := os.Mkdir(*directory, 0700); err != nil {
		log.Fatal("Certificate directory must be new; existing files are not overwritten")
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal("Key generation failed")
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		log.Fatal("Serial generation failed")
	}
	template := x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Dapper local development"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(30 * 24 * time.Hour), DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	certificate, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		log.Fatal("Certificate generation failed")
	}
	if err = os.WriteFile(filepath.Join(*directory, "certificate.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}), 0600); err != nil {
		log.Fatal("Certificate file write failed")
	}
	if err = os.WriteFile(filepath.Join(*directory, "private-key.pem"), pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600); err != nil {
		log.Fatal("Private key file write failed")
	}
	log.Print("Local development certificate files generated; no system trust settings changed")
}
