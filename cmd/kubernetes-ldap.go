package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"github.com/golang/glog"

	"kubernetes-ldap/auth"
	"kubernetes-ldap/ldap"
	"kubernetes-ldap/token"

	"flag"
)

const (
	usage = "kubernetes-ldap <options>"
	version = "v1.0.0"
)

// Define input flags
// LDAP options
var flLdapUseInsecure = flag.Bool("ldap-insecure", false, "Disable LDAP TLS")
var flLdapHost = flag.String("ldap-host", "", "Host or IP of the LDAP server")
var flLdapPort = flag.Uint("ldap-port", 389, "LDAP server port")
var flBaseDN = flag.String("ldap-base-dn", "", "LDAP user base DN in the form 'dc=example,dc=com'")
var flUserLoginAttribute = flag.String("ldap-user-attribute", "uid", "LDAP Username attribute for login")
var flSearchUserDN = flag.String("ldap-search-user-dn", "", "Search user DN for this app to find users (e.g.: cn=admin,dc=example,dc=com).")
var flSearchUserPassword = flag.String("ldap-search-user-password", "", "Search user password")
var flSkipLdapTLSVerification = flag.Bool("ldap-skip-tls-verification", false, "Skip LDAP server TLS verification")
var flGroupFilter = flag.String("group-filter","","Regex to filter group membership")
var flUsernameAttribute = flag.String("token-username-attribute","mail","LDAP attribute to use for username in token")

// Token options
var flTokenExpireTime = flag.Int("token-expire-time",12,"Time in hours the issued token is valid")

// webhook http(s) server options
var flServerPort = flag.Uint("port", 4000, "Local port this proxy server will run on")
var flhHealthzPort = flag.Uint("health-port", 8080, "port to server readynessprobe on")
var flTLSCertFile = flag.String("tls-cert-file", "",
	"File containing x509 Certificate for HTTPS.  (CA cert, if any, concatenated after server cert).")
var flTLSPrivateKeyFile = flag.String("tls-private-key-file", "", "File containing x509 private key matching --tls-cert-file.")
var flUseTls = flag.Bool("use-tls",true,"Use tls for webhook server")

// other flags
var flVersion = flag.Bool("version",false,"print version and exit")

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s\n", usage)
		flag.PrintDefaults()
	}
}

func main() {
	// parse the imput flags
	flag.Parse()

	//print verion and exit
	if *flVersion {
		fmt.Printf("Kubernetes-Ldap webhook server version: %s\n",version)
		os.Exit(0)
	}

	// validate required flags
	requireFlag("--ldap-host", flLdapHost)
	requireFlag("--ldap-base-dn", flBaseDN)
	if *flUseTls {
		requireFlag("--tls-cert-file", flTLSCertFile)
		requireFlag("--tls-private-key", flTLSPrivateKeyFile)
	}

	glog.CopyStandardLogTo("INFO")

	keypairFilename := "signing"
	glog.Info("Generating token singing keypair")
	if err := token.GenerateKeypair(keypairFilename); err != nil {
		glog.Errorf("Error generating key pair: %v", err)
	}

	var err error
	tokenSigner, err := token.NewSigner(keypairFilename)
	if err != nil {
		glog.Errorf("Error creating token issuer: %v", err)
	}

	tokenVerifier, err := token.NewVerifier(keypairFilename)
	if err != nil {
		glog.Errorf("Error creating token verifier: %v", err)
	}

	ldapTLSConfig := &tls.Config{
		ServerName:         *flLdapHost,
		InsecureSkipVerify: *flSkipLdapTLSVerification,
	}

	ldapClient := &ldap.Client{
		BaseDN:             *flBaseDN,
		LdapServer:         *flLdapHost,
		LdapPort:           *flLdapPort,
		UseInsecure:        *flLdapUseInsecure,
		UserLoginAttribute: *flUserLoginAttribute,
		SearchUserDN:       *flSearchUserDN,
		SearchUserPassword: *flSearchUserPassword,
		TLSConfig:          ldapTLSConfig,
	}

	publicRouter := http.NewServeMux()
	sslRouter := http.NewServeMux()

	webhook := auth.NewTokenWebhook(tokenVerifier)

	ldapTokenIssuer := &auth.LDAPTokenIssuer{
		LDAPAuthenticator: 	ldapClient,
		TokenSigner:       	tokenSigner,
		GroupFilter:       	*flGroupFilter,
		ExpireTime:        	*flTokenExpireTime,
		UsernameLDAPAttribute: 	*flUsernameAttribute,
	}

	// Endpoint for authenticating with token
	publicRouter.Handle("/authenticate", webhook)

	// Endpoint for token issuance after LDAP auth
	publicRouter.Handle("/ldapAuth", ldapTokenIssuer)

	// Endpoint for healthz on ssl port
	publicRouter.HandleFunc("/healthz", healthz)

	TLSConfig := &tls.Config{
		// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
		MinVersion: tls.VersionTLS10,
	}

	//setting up servers
	sslServer := &http.Server{
		Addr: fmt.Sprintf(":%d", *flServerPort),
		Handler: sslRouter,
		TLSConfig: TLSConfig,
	}

	publicServer := &http.Server{
		Addr: fmt.Sprintf(":%d", *flhHealthzPort),
		Handler: publicRouter,
		TLSConfig: TLSConfig,
	}

	// starting public server
	go publicServer.ListenAndServe()
	glog.Infof("Serving /healthz on %s", fmt.Sprintf(":%d", *flhHealthzPort))
	// starting api server
	glog.Infof("Serving /authenticate on %s", fmt.Sprintf(":%d", *flhHealthzPort))
	glog.Infof("Serving /ldapAuth on %s", fmt.Sprintf(":%d", *flhHealthzPort))
	if *flUseTls {
		glog.Fatal(sslServer.ListenAndServeTLS(*flTLSCertFile, *flTLSPrivateKeyFile))
	} else {
		glog.Fatal(sslServer.ListenAndServe())
	}

}

func requireFlag(flagName string, flagValue *string) {
	if *flagValue == "" {
		fmt.Fprintf(os.Stderr, "kubernetes-ldap: %s is required. \nUse -h flag for help.\n", flagName)
		os.Exit(1)
	}
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
