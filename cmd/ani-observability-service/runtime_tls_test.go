package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	conf "github.com/zhangzhe-ctrl/ani-observability-service/internal/conf/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

type runtimeTLS struct {
	serverCert, serverKey, ca string
	client                    *tls.Config
}

func (m runtimeTLS) governance() *conf.Governance {
	return &conf.Governance{TlsCertFile: m.serverCert, TlsKeyFile: m.serverKey, ClientCaFile: m.ca}
}

func runtimeMonitoring(address string) *conf.Monitoring {
	return &conf.Monitoring{PrometheusTimeout: durationpb.New(4 * time.Second), MaxConcurrentRequests: 16, MaxConcurrentRequestsPerTenant: 4,
		Datasources: []*conf.Datasource{{ClusterId: "0fcdb8b1-22bf-4528-af59-82843dc88f21", Address: address}}}
}

func newRuntimeTLS(t *testing.T) runtimeTLS {
	t.Helper()
	directory := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "runtime test CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	ca, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	caPath := filepath.Join(directory, "ca.crt")
	if err := os.WriteFile(caPath, caPEM, 0600); err != nil {
		t.Fatal(err)
	}
	issue := func(name string, usage x509.ExtKeyUsage) (string, string) {
		t.Helper()
		leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
		if err != nil {
			t.Fatal(err)
		}
		leaf := &x509.Certificate{SerialNumber: serial, DNSNames: []string{name}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
		certificate, err := x509.CreateCertificate(rand.Reader, leaf, ca, &leafKey.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		privateKey, err := x509.MarshalPKCS8PrivateKey(leafKey)
		if err != nil {
			t.Fatal(err)
		}
		certPath, keyPath := filepath.Join(directory, name+".crt"), filepath.Join(directory, name+".key")
		if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKey}), 0600); err != nil {
			t.Fatal(err)
		}
		return certPath, keyPath
	}
	serverCert, serverKey := issue("ani-observability-service", x509.ExtKeyUsageServerAuth)
	clientCert, clientKey := issue("ani-governance", x509.ExtKeyUsageClientAuth)
	pair, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(caPEM)
	return runtimeTLS{serverCert: serverCert, serverKey: serverKey, ca: caPath, client: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, ServerName: "ani-observability-service", Certificates: []tls.Certificate{pair}}}
}
