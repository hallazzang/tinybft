package types

import (
	"bytes"
	"encoding/gob"
)

type Proposal struct {
	Height    int64
	Round     int32
	POLRound  int32
	BlockID   BlockID
	Signature []byte
}

func NewProposal(height int64, round, polRound int32, blockID BlockID) *Proposal {
	return &Proposal{
		Height:   height,
		Round:    round,
		POLRound: polRound,
		BlockID:  blockID,
	}
}

func (p *Proposal) SignBytes() []byte {
	buf := &bytes.Buffer{}
	if err := gob.NewEncoder(buf).Encode(p); err != nil {
		panic("encode vote")
	}
	return buf.Bytes()
}
