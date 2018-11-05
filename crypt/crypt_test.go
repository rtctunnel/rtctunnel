package crypt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	k1 := GenerateKeyPair()
	k2 := GenerateKeyPair()

	msg := []byte("Hello World")

	encrypted := k1.Encrypt(k2.Public, msg)
	decrypted, err := k2.Decrypt(k1.Public, encrypted)
	assert.NoError(t, err)
	assert.Equal(t, msg, decrypted)
}
