package types

import (
	"crypto/ed25519"
	"crypto/rand"
)

type Validator struct {
	Address     Address
	PubKey      PubKey
	VotingPower int64
}

func NewValidator(pubKey PubKey, votingPower int64) *Validator {
	return &Validator{
		Address:     pubKey.Address(),
		PubKey:      pubKey,
		VotingPower: votingPower,
	}
}

func (val *Validator) Copy() *Validator {
	v := *val
	return &v
}

type PrivValidator struct {
	PrivKey ed25519.PrivateKey
}

func NewPrivValidator() *PrivValidator {
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	return &PrivValidator{
		PrivKey: privKey,
	}
}

func (pv *PrivValidator) SignVote(vote *Vote) error {
	signBytes := vote.SignBytes()
	sig := ed25519.Sign(pv.PrivKey, signBytes)
	vote.Signature = sig
	return nil
}

func (pv *PrivValidator) SignProposal(proposal *Proposal) interface{} {
	signBytes := proposal.SignBytes()
	sig := ed25519.Sign(pv.PrivKey, signBytes)
	proposal.Signature = sig
	return nil
}
