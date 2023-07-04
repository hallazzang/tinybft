package types

import (
	"crypto/ed25519"
	"crypto/sha256"
)

var _ PubKey = (*ed25519PubKey)(nil)

type PubKey interface {
	VerifySignature(msg, sig []byte) bool
	Address() Address
}

type ed25519PubKey struct {
	pubKey ed25519.PublicKey
}

func (pubKey *ed25519PubKey) VerifySignature(msg, sig []byte) bool {
	return ed25519.Verify(pubKey.pubKey, msg, sig)
}

func (pubKey *ed25519PubKey) Address() Address {
	sum := sha256.Sum256(pubKey.pubKey)
	return sum[:]
}
