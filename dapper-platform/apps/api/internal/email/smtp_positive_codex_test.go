package email

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

func TestCodexSMTPVerifiedSTARTTLSDelivery(t *testing.T) {
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "synthetic-loopback"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	cert, e := x509.ParseCertificate(der)
	if e != nil {
		t.Fatal(e)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	result := make(chan error, 1)
	go func() {
		c, e := listener.Accept()
		if e != nil {
			result <- e
			return
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(3 * time.Second))
		reader := bufio.NewReader(c)
		fmt.Fprint(c, "220 synthetic\r\n")
		encrypted := false
		for {
			line, e := reader.ReadString('\n')
			if e != nil {
				result <- e
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				fmt.Fprint(c, "250-synthetic\r\n250 STARTTLS\r\n")
			case strings.HasPrefix(line, "STARTTLS"):
				fmt.Fprint(c, "220 upgrade\r\n")
				secured := tls.Server(c, &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12})
				if e = secured.Handshake(); e != nil {
					result <- e
					return
				}
				c = secured
				reader = bufio.NewReader(c)
				encrypted = true
			case strings.HasPrefix(line, "MAIL") || strings.HasPrefix(line, "RCPT"):
				if !encrypted {
					result <- fmt.Errorf("plaintext envelope")
					return
				}
				fmt.Fprint(c, "250 OK\r\n")
			case strings.HasPrefix(line, "DATA"):
				if !encrypted {
					result <- fmt.Errorf("plaintext message")
					return
				}
				fmt.Fprint(c, "354 send\r\n")
				var body strings.Builder
				for {
					part, e := reader.ReadString('\n')
					if e != nil {
						result <- e
						return
					}
					if part == ".\r\n" {
						break
					}
					body.WriteString(part)
				}
				if !strings.Contains(body.String(), "Synthetic TLS body") {
					result <- fmt.Errorf("missing body")
					return
				}
				fmt.Fprint(c, "250 accepted\r\n")
				result <- nil
				return
			default:
				result <- fmt.Errorf("unexpected SMTP command")
				return
			}
		}
	}()
	sender, e := NewSMTPSender(Config{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, FromAddress: "sender@example.test"})
	if e != nil {
		t.Fatal(e)
	}
	sender.tlsRoots = roots
	ref, e := sender.SendEmail(context.Background(), "recipient@example.test", "Synthetic TLS subject", "", "Synthetic TLS body")
	if e != nil {
		t.Fatal(e)
	}
	if ref == "" {
		t.Fatal("missing submission reference")
	}
	if e = <-result; e != nil {
		t.Fatal(e)
	}
}
