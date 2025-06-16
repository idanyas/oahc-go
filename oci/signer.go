package oci

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Signer is responsible for signing OCI API requests.
type Signer struct {
	keyID      string
	privateKey *rsa.PrivateKey
}

// NewSigner creates a new Signer.
func NewSigner(tenancyID, userID, fingerprint, privateKeyPath string) (*Signer, error) {
	keyID := fmt.Sprintf("%s/%s/%s", tenancyID, userID, fingerprint)

	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("could not read private key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}

	var privateKey *rsa.PrivateKey
	// Try parsing as PKCS1 first, then PKCS8.
	pkcs1Key, err1 := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err1 == nil {
		privateKey = pkcs1Key
	} else {
		pkcs8Key, err8 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err8 != nil {
			return nil, fmt.Errorf("failed to parse private key: (pkcs1: %v), (pkcs8: %v)", err1, err8)
		}
		var ok bool
		privateKey, ok = pkcs8Key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("key is not an RSA private key")
		}
	}

	return &Signer{
		keyID:      keyID,
		privateKey: privateKey,
	}, nil
}

// Sign adds the necessary signing headers to an HTTP request.
func (s *Signer) Sign(req *http.Request, body []byte) error {
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)

	// Construct signing string
	var headersToSign []string
	var signingString string

	// (request-target)
	requestTarget := fmt.Sprintf("%s %s", strings.ToLower(req.Method), req.URL.RequestURI())
	signingString += fmt.Sprintf("(request-target): %s", requestTarget)
	headersToSign = append(headersToSign, "(request-target)")

	// date
	signingString += fmt.Sprintf("\ndate: %s", date)
	headersToSign = append(headersToSign, "date")

	// host
	host := req.URL.Host
	req.Header.Set("Host", host)
	signingString += fmt.Sprintf("\nhost: %s", host)
	headersToSign = append(headersToSign, "host")

	// Handle body headers
	if req.Method == "POST" || req.Method == "PUT" {
		contentType := "application/json"
		req.Header.Set("Content-Type", contentType)

		// x-content-sha256
		hash := sha256.Sum256(body)
		contentSha256 := base64.StdEncoding.EncodeToString(hash[:])
		req.Header.Set("x-content-sha256", contentSha256)

		// content-length
		contentLength := fmt.Sprintf("%d", len(body))
		req.Header.Set("Content-Length", contentLength)

		signingString += fmt.Sprintf("\nx-content-sha256: %s", contentSha256)
		signingString += fmt.Sprintf("\ncontent-type: %s", contentType)
		signingString += fmt.Sprintf("\ncontent-length: %s", contentLength)

		headersToSign = append(headersToSign, "x-content-sha256", "content-type", "content-length")
	}

	// Sign the string
	hasher := sha256.New()
	hasher.Write([]byte(signingString))
	hashed := hasher.Sum(nil)

	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, hashed)
	if err != nil {
		return fmt.Errorf("failed to sign string: %w", err)
	}
	encodedSignature := base64.StdEncoding.EncodeToString(signature)

	// Construct Authorization header
	authHeader := fmt.Sprintf(
		`Signature version="1",keyId="%s",algorithm="rsa-sha256",headers="%s",signature="%s"`,
		s.keyID,
		strings.Join(headersToSign, " "),
		encodedSignature,
	)

	req.Header.Set("Authorization", authHeader)
	return nil
}
