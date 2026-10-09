package checker

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/cng1985/nwatch/internal/model"
)

func TestTLSCheckerValidAndExpired(t *testing.T) {
	valid := serveTLS(t, "example.test", time.Now().Add(-time.Hour), time.Now().Add(40*24*time.Hour))
	expired := serveTLS(t, "example.test", time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour))

	c := NewTLSChecker()
	c.roots = valid.roots
	res, err := c.Check(context.Background(), &model.Monitor{
		URL: valid.addr, Timeout: 3, TLSWarningDays: 14, TLSCriticalDays: 7,
	})
	if err != nil || !res.Success || res.TLS == nil || res.TLS.Status != model.TLSNormal {
		t.Fatalf("valid %+v %v", res, err)
	}
	if res.TLS.CN != "example.test" || res.TLS.DaysRemaining < 39 {
		t.Fatalf("cert meta %+v", res.TLS)
	}

	c.roots = expired.roots
	res, err = c.Check(context.Background(), &model.Monitor{URL: expired.addr, Timeout: 3, TLSWarningDays: 14, TLSCriticalDays: 7})
	if err != nil || res.Success || res.TLS == nil || res.TLS.Status != model.TLSExpired {
		t.Fatalf("expired %+v %v", res, err)
	}
}

func TestTLSWarningWindow(t *testing.T) {
	leaf := serveTLS(t, "soon.test", time.Now().Add(-time.Hour), time.Now().Add(6*24*time.Hour))
	c := NewTLSChecker()
	c.roots = leaf.roots
	res, err := c.Check(context.Background(), &model.Monitor{URL: leaf.addr, Timeout: 3, TLSWarningDays: 14, TLSCriticalDays: 7})
	if err != nil || !res.Success || res.TLS.Status != model.TLSCritical {
		t.Fatalf("%+v %v", res, err)
	}
}

type tlsFixture struct {
	addr  string
	roots *x509.CertPool
}

func serveTLS(t *testing.T, name string, notBefore, notAfter time.Time) tlsFixture {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: name},
		DNSNames:     []string{name},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	keyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{pair}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				if tc, ok := conn.(*tls.Conn); ok {
					_ = tc.Handshake()
				}
			}()
		}
	}()
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	return tlsFixture{addr: ln.Addr().String(), roots: pool}
}

func TestTLSEndpointParse(t *testing.T) {
	host, port, name := TLSEndpoint(&model.Monitor{URL: "https://api.example.com/health"})
	if host != "api.example.com" || port != "443" || name != "api.example.com" {
		t.Fatalf("%s %s %s", host, port, name)
	}
	host, port, _ = TLSEndpoint(&model.Monitor{Host: "10.0.0.8", Port: 8443})
	if host != "10.0.0.8" || port != "8443" {
		t.Fatalf("%s %s", host, port)
	}
	_ = net.JoinHostPort(host, port)
}
