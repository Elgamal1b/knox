package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/user"
	"time"

	"github.com/pinterest/knox"
	"github.com/pinterest/knox/client"
)

// certPEMBlock is the certificate signed by the CA to identify the machine using the client
// (Should be pulled from a file or via another process)
const certPEMBlock = `-----BEGIN CERTIFICATE-----
MIIB7TCCAZOgAwIBAgIDEAAEMAoGCCqGSM49BAMCMFExCzAJBgNVBAYTAlVTMQsw
CQYDVQQIEwJDQTEYMBYGA1UEChMPTXkgQ29tcGFueSBOYW1lMRswGQYDVQQDExJ1
c2VPbmx5SW5EZXZPclRlc3QwHhcNMTgwMzAyMDI1NjEyWhcNMTkwMzAyMDI1NjEy
WjBKMQswCQYDVQQGEwJVUzELMAkGA1UECAwCQ0ExGDAWBgNVBAoMD015IENvbXBh
bnkgTmFtZTEUMBIGA1UEAwwLZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggqhkjO
PQMBBwNCAAQQTbdQNoE5/j6mgh4HAdbgPyGbuzjpHI/x34p6qPojduUK+ifUW6Mb
bS5Zumjh31K5AmWYt4jWfU82Sb6sxPKXo2EwXzAJBgNVHRMEAjAAMAsGA1UdDwQE
AwIF4DBFBgNVHREEPjA8hhxzcGlmZmU6Ly9leGFtcGxlLmNvbS9zZXJ2aWNlggtl
eGFtcGxlLmNvbYIPd3d3LmV4YW1wbGUuY29tMAoGCCqGSM49BAMCA0gAMEUCIQDO
TaI0ltMPlPDt4XSdWJawZ4euAGXJCyoxHFs8HQK8XwIgVokWyTcajFoP0/ZfzrM5
SihfFJr39Ck4V5InJRHPPtY=
-----END CERTIFICATE-----`

// keyPEMBlock is the private key that should only be available on the machine running this client
// (Should be pulled from a file or via another process)
const keyPEMBlock = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIDHDjs9Ug8QvsuKRrtC6QUmz4u++oBJF2VtCZe9gYyzOoAoGCCqGSM49
AwEHoUQDQgAEEE23UDaBOf4+poIeBwHW4D8hm7s46RyP8d+Keqj6I3blCvon1Fuj
G20uWbpo4d9SuQJlmLeI1n1PNkm+rMTylw==
-----END EC PRIVATE KEY-----`

// hostname is the host running the knox server
const hostname = "localhost:9000"

// tokenEndpoint and clientID are used by "knox login" if your oauth client supports password flows.
const tokenEndpoint = "https://oauth.token.endpoint.used.for/knox/login"
const clientID = ""

// keyFolder is the directory where keys are cached
const keyFolder = "/var/lib/knox/v0/keys/"

// authTokenResp is the format of the OAuth response generated by "knox login"
type authTokenResp struct {
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
}

// getCert returns the cert in the tls.Certificate format. This should be a config option in prod.
func getCert() (tls.Certificate, error) {
	return tls.X509KeyPair([]byte(certPEMBlock), []byte(keyPEMBlock))
}

// authHandler is used to generate an authentication header.
// The server expects VersionByte + TypeByte + IDToPassToAuthHandler.
func authHandler() string {
	if s := os.Getenv("KNOX_USER_AUTH"); s != "" {
		return "0u" + s
	}
	if s := os.Getenv("KNOX_MACHINE_AUTH"); s != "" {
		c, _ := getCert()
		x509Cert, err := x509.ParseCertificate(c.Certificate[0])
		if err != nil {
			return "0t" + s
		}
		if len(x509Cert.Subject.CommonName) > 0 {
			return "0t" + x509Cert.Subject.CommonName
		} else if len(x509Cert.DNSNames) > 0 {
			return "0t" + x509Cert.DNSNames[0]
		} else {
			return "0t" + s
		}
	}
	if s := os.Getenv("KNOX_SERVICE_AUTH"); s != "" {
		return "0s" + s
	}
	u, err := user.Current()
	if err != nil {
		return ""
	}

	d, err := ioutil.ReadFile(u.HomeDir + "/.knox_user_auth")
	if err != nil {
		return ""
	}
	var authResp authTokenResp
	err = json.Unmarshal(d, &authResp)
	if err != nil {
		return ""
	}

	return "0u" + authResp.AccessToken
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	tlsConfig := &tls.Config{
		ServerName:         "knox",
		InsecureSkipVerify: true,
	}

	cert, err := getCert()
	if err == nil {
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	cli := &knox.HTTPClient{
		Host:        hostname,
		AuthHandler: authHandler,
		KeyFolder:   keyFolder,
		Client:      &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}},
	}

	loginCommand := client.NewLoginCommand(clientID, tokenEndpoint, "", "", "", "")

	client.Run(
		cli,
		&client.VisibilityParams{
			Logf:    log.Printf,
			Errorf:  log.Printf,
			SummaryMetrics: func(map[string]uint64) {},
			InvokeMetrics: func(map[string]string) {},
		},
		loginCommand,
	)
}
