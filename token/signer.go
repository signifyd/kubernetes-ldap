package token

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"gopkg.in/square/go-jose.v2"
)

// Signer signs an issued token
type Signer interface {
	// Sign a token and return the serialized cryptographic token.
	Sign(token *AuthToken) (string, error)
}

// rsaSigner represents a signer of tokens under a particular public key.
type rsaSigner struct {
	rsaVerifier
	signer jose.Signer
}

// NewSigner is, for the moment, a thin wrapper around Square's
// go-jose library to issue RSA-PS512 JWS tokens.
func NewSigner(filename string) (Signer, error) {
	key, err := ioutil.ReadFile(filename + ".priv")
	if err != nil {
		return nil, err
	}

	privateKey, err := LoadPrivateKey(key)
	if err != nil {
		return nil, err
	}

	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("expected an RSA private key, but got a key of type %T", privateKey)
	}

	signer, err := jose.NewSigner(jose.SigningKey{jose.PS512, privateKey}, nil)
	if err != nil {
		return nil, err
	}
	ecdsaSigner := &rsaSigner{
		signer: signer,
	}
	ecdsaSigner.publicKey = &rsaKey.PublicKey
	return ecdsaSigner, nil
}

// Sign an authentcation token and return the serialized JWS
func (es *rsaSigner) Sign(token *AuthToken) (string, error) {
	tokenBytes, err := json.Marshal(token)
	if err != nil {
		// panic? what are the conditions under which this can fail?
		return "", err
	}
	jws, err := es.signer.Sign(tokenBytes)
	if err != nil {
		return "", err
	}
	signed, err := jws.CompactSerialize()
	if err != nil {
		return "", err
	}
	return signed, nil
}
