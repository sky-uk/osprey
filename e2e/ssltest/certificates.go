package ssltest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"
)

// CreateCertificates creates an RSA private key and a self signed certificate to use for TLS.
// Writes the pem encoded files to disk and returns the locations for the files.
func CreateCertificates(cn, destDir string) (certFile, keyFile string) {
	keyFile = fmt.Sprintf("%s/%sserver.key", destDir, cn)
	certFile = fmt.Sprintf("%s/%sserver.crt", destDir, cn)

	if err := checkPemPair(keyFile, certFile); err != nil {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			panic(fmt.Sprintf("Failed to create certificates dir %s: %v", destDir, err))
		}
		template := &x509.Certificate{
			IsCA: true,
			BasicConstraintsValid: true,
			SerialNumber:          big.NewInt(1),
			Subject: pkix.Name{
				CommonName: cn,
			},

			NotBefore: time.Now(),
			NotAfter:  time.Now().Add(1 * time.Hour),

			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		}

		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			fmt.Println(err)
		}
		// create a self-signed certificate.
		crt, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
		if err != nil {
			fmt.Println(err)
		}

		writePem(keyFile, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(privateKey))
		writePem(certFile, "CERTIFICATE", crt)
	}

	return certFile, keyFile
}

func writePem(filename, pemType string, content []byte) {
	pemFile, err := os.Create(filename)
	if err != nil {
		panic(fmt.Sprintf("Failed to create pem file dir %s: %v", filename, err))
	}
	var pemKey = &pem.Block{
		Type:  pemType,
		Bytes: content,
	}
	pem.Encode(pemFile, pemKey)
	pemFile.Close()
}

func checkPemPair(pems ...string) error {
	for _, pem := range pems {
		if _, err := os.Stat(pem); os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
