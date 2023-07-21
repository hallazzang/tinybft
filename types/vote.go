package types

import (
	"encoding/json"
	"errors"
	"fmt"
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

func (vote *Vote) Canonicalize() CanonicalVote {
	return CanonicalVote{
		Type:      vote.Type,
		Height:    vote.Height,
		Round:     vote.Round,
		BlockID:   vote.BlockID,
		Timestamp: vote.Timestamp,
	}
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
	pb := vote.Canonicalize()
	bz, err := json.Marshal(pb)
	if err != nil {
		panic(fmt.Errorf("marshal vote: %w", err))
	}
	return bz
}

type CanonicalVote struct {
	Type      VoteType
	Height    int64
	Round     int32
	BlockID   BlockID
	Timestamp time.Time
}
