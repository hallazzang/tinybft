package types

import (
	"bytes"
	"encoding/gob"
	"errors"
	"time"
)

type VoteType int

const (
	Prevote   = VoteType(1)
	Precommit = VoteType(2)
)

type Vote struct {
	Type             VoteType
	Height           int64
	Round            int32
	BlockID          BlockID
	Timestamp        time.Time
	ValidatorAddress Address
	ValidatorIndex   int32
	Signature        []byte
}

func (vote *Vote) Verify(pubKey PubKey) error {
	if !pubKey.Address().Equal(vote.ValidatorAddress) {
		return errors.New("invalid validator address")
	}
	if !pubKey.VerifySignature(vote.SignBytes(), vote.Signature) {
		return errors.New("invalid signature")
	}
	return nil
}

func (vote *Vote) SignBytes() []byte {
	buf := &bytes.Buffer{}
	if err := gob.NewEncoder(buf).Encode(vote); err != nil {
		panic("encode vote")
	}
	return buf.Bytes()
}
