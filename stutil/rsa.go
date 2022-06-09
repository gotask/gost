package stutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

/** public kye
-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIBolLZTpZadc4GNADCBiQKBgQDQZLw8KTD4uFmKsrUuCCt3bn/u
+bPNE15R6tsTid0E+dSopxqIqmJ+eTAvgxcSeMsKqfd9R8r+0mk6u+FWr9GtcCdX
3B2VtWAsI5FQBvDUISO+Q5LpABolLZTpZflgN901LLF5fXW4Qcc9ME64zmkeSapL
QEjd0E+dSopxqIqmJwIDAQAB
-----END PUBLIC KEY-----
**/

func RsaDecrypt(ciphertext []byte, pri_key []byte) ([]byte, error) {

	block, _ := pem.Decode(pri_key)
	if block == nil {
		return nil, errors.New("invalid rsa private key")
	}

	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return rsa.DecryptPKCS1v15(rand.Reader, priv, ciphertext)
}

func RsaEncrypt(plaintext []byte, pub_key []byte) ([]byte, error) {
	block, _ := pem.Decode(pub_key)
	if block == nil {
		return nil, errors.New("invalid rsa public key")
	}

	pubInf, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub := pubInf.(*rsa.PublicKey)
	return rsa.EncryptPKCS1v15(rand.Reader, pub, plaintext)
}
