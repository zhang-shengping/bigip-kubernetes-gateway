package helpers

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

func TestGenerateCertificateWithDefault(t *testing.T) {
	crts := NewCertificates()

	capem, caprivatekey, err := crts.GenerateCA()
	if err != nil {
		t.Errorf("Default GenerateCA fail: %s\n", err)
	}

	serverpem, serverprivatekey, err := crts.GenerateServerCert(capem, caprivatekey)
	if err != nil {
		t.Errorf("Default ServerCA fail: %s\n", err)
	}

	err = VerifyServerWithCA(capem, serverpem)
	if err != nil {
		t.Errorf("Verfiy Server certificate fail: Server certificate:\n %s Server PrivateKey:\n %s \n %s",
			string(serverpem), string(serverprivatekey), err)
	}
}

func TestGenerateDefaultServerCert(t *testing.T) {

	capkname := pkix.Name{
		Organization:  []string{"Test"},
		Country:       []string{"CN"},
		Province:      []string{""},
		Locality:      []string{"Seattle"},
		StreetAddress: []string{"WA Corporate HQ 801 5th Ave"},
		PostalCode:    []string{"98104"},
	}

	serverpkname := pkix.Name{
		Organization:  []string{"Test, Dev."},
		Country:       []string{"CN"},
		Province:      []string{""},
		Locality:      []string{"Hefei"},
		StreetAddress: []string{"Jiang Guo Road"},
		PostalCode:    []string{"100000"},
	}

	crts := NewCertificates(
		SetCaCrt(&x509.Certificate{
			SerialNumber:          big.NewInt(2019),
			Subject:               capkname,
			NotBefore:             time.Now(),
			NotAfter:              time.Now().AddDate(10, 0, 0),
			IsCA:                  true,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
			KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			BasicConstraintsValid: true,
		}),
		SetServerCrt(&x509.Certificate{
			SerialNumber: big.NewInt(2019),
			Subject:      serverpkname,
			IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
			NotBefore:    time.Now(),
			NotAfter:     time.Now().AddDate(10, 0, 0),
			SubjectKeyId: []byte{1, 2, 3, 4, 5},
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
			KeyUsage:     x509.KeyUsageDigitalSignature,
		}),
	)
	if crts.CAcrt.Subject.Organization[0] != capkname.Organization[0] {
		t.Errorf("Customized CA Certificate Defination is not set.\n want:\n %s \n got:\n %s \n",
			capkname, crts.CAcrt.Subject.Organization)
	}

	if crts.Servercrt.Subject.Organization[0] != serverpkname.Organization[0] {
		t.Errorf("Customized Server Certificate Defination is not set.\n want:\n %s \n got:\n %s \n",
			serverpkname, crts.Servercrt.Subject.Organization)
	}

	capem, caprivatekey, err := crts.GenerateCA()
	if err != nil {
		t.Errorf("Customized GenerateCA fail: %s\n", err)
	}

	serverpem, serverprivatekey, err := crts.GenerateServerCert(capem, caprivatekey)
	if err != nil {
		t.Errorf("Customized ServerCA fail: %s\n", err)
	}

	err = VerifyServerWithCA(capem, serverpem)
	if err != nil {
		t.Errorf("Verfiy Customized Server certificate fail: Server certificate:\n %s Server PrivateKey:\n %s \n %s",
			string(serverpem), string(serverprivatekey), err)
	}
}
