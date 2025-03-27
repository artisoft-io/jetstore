package main

// Generate self signed cert for api server
// from https://cs.opensource.google/go/go/+/refs/tags/go1.24.1:src/crypto/tls/generate_cert.go

// Generate a self-signed X.509 certificate for a TLS server. Outputs to
// 'cert.pem' and 'key.pem' and will overwrite existing files.

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

func GenerateCert() error {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %v", err)
	}

	// RSA subject keys should have the DigitalSignature
	// KeyUsage bits set in the x509.Certificate template
	// Only RSA subject keys should have the KeyEncipherment KeyUsage bits set. In
	// the context of TLS this KeyUsage is particular to RSA key exchange and
	// authentication.
	keyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	if *usingSshTunnel {
		template.IPAddresses = append(template.IPAddresses, net.ParseIP("127.0.0.1"))
	} else {
		ip, err := awsi.GetPrivateIp()
		if err != nil {
			return fmt.Errorf("while getting local private ip: %v", err)
		}
		template.IPAddresses = append(template.IPAddresses, net.ParseIP(ip))
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %v", err)
	}

	certOut, err := os.Create("cert.pem")
	if err != nil {
		return fmt.Errorf("failed to open cert.pem for writing: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("failed to write data to cert.pem: %v", err)
	}
	if err := certOut.Close(); err != nil {
		return fmt.Errorf("error closing cert.pem: %v", err)
	}
	log.Print("wrote cert.pem\n")

	keyOut, err := os.OpenFile("key.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open key.pem for writing: %v", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("unable to marshal private key: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("failed to write data to key.pem: %v", err)
	}
	if err := keyOut.Close(); err != nil {
		return fmt.Errorf("error closing key.pem: %v", err)
	}
	log.Print("wrote key.pem\n")
	return nil
}
