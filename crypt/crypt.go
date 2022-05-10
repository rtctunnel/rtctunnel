package crypt

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"

	"github.com/mr-tron/base58"
	"golang.org/x/crypto/nacl/box"
)

const (
	// KeySize is the size of an encryption key in bytes
	KeySize = 32
	// NonceSize is the size of a nonce in bytes
	NonceSize = 24
)

type (
	// Key is a public or private encryption key
	Key [KeySize]byte
	// A KeyPair is a public, private key pair
	KeyPair struct {
		Public, Private Key
	}
	// Nonce is a number used once
	Nonce = [NonceSize]byte
)

// NewKey creates a new key from a base58 string
func NewKey(str string) (key Key, err error) {
	bs, err := base58.Decode(str)
	if err != nil {
		return key, err
	}
	if len(bs) != KeySize {
		return key, errors.New("invalid key")
	}
	copy(key[:], bs)
	return key, nil
}

func (key Key) Valid() bool {
	return [KeySize]byte(key) != [KeySize]byte{}
}

func (key Key) String() string {
	return base58.Encode(key[:])
}

func (key Key) MarshalJSON() ([]byte, error) {
	bs, err := json.Marshal(key.String())
	if err != nil {
		return nil, err
	}
	return bs, nil
}

func (key *Key) UnmarshalJSON(data []byte) error {
	var raw string
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
	}
	*key, err = NewKey(raw)
	if err != nil {
		return err
	}
	return nil
}

func (key Key) MarshalYAML() (interface{}, error) {
	return key.String(), nil
}

func (key *Key) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var str string
	err := unmarshal(&str)
	if err != nil {
		return err
	}
	*key, err = NewKey(str)
	if err != nil {
		return err
	}
	return nil
}

// GenerateKeyPair generates a (public, private) encryption key
func GenerateKeyPair() KeyPair {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	return KeyPair{Public: *pub, Private: *priv}
}

// Encrypt encrypts a message using a peer's public key and the local private key
func (pair *KeyPair) Encrypt(peerPublicKey Key, data []byte) []byte {
	nonce := generateNonce()
	k1 := [KeySize]byte(peerPublicKey)
	k2 := [KeySize]byte(pair.Private)
	sealed := box.Seal(nil, data, &nonce, &k1, &k2)

	var result []byte
	result = append(result, nonce[:]...)
	result = append(result, sealed...)
	return result
}

// Decrypt decrypts a message using a peer's public key and the local private key
func (pair *KeyPair) Decrypt(peerPublicKey Key, data []byte) ([]byte, error) {
	if len(data) < NonceSize {
		return nil, errors.New("invalid message")
	}
	var nonce Nonce
	copy(nonce[:], data)
	sealed := data[NonceSize:]
	k1 := [KeySize]byte(peerPublicKey)
	k2 := [KeySize]byte(pair.Private)
	opened, ok := box.Open(nil, sealed, &nonce, &k1, &k2)
	if !ok {
		return nil, errors.New("invalid message")
	}

	return opened, nil
}

func generateNonce() Nonce {
	var nonce Nonce
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		panic(err)
	}
	return nonce
}
