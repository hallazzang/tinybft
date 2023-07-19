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
	Block     *Block
	Signature []byte
}

func NewProposal(height int64, round, polRound int32, blockID BlockID, block *Block) *Proposal {
	return &Proposal{
		Height:   height,
		Round:    round,
		POLRound: polRound,
		BlockID:  blockID,
		Block:    block,
	}
}

func (p *Proposal) Canonicalize() CanonicalProposal {
	return CanonicalProposal{
		Height:   p.Height,
		Round:    p.Round,
		POLRound: p.POLRound,
		BlockID:  p.BlockID,
		Block:    p.Block,
	}
}

func (p *Proposal) SignBytes() []byte {
	pb := p.Canonicalize()
	buf := &bytes.Buffer{}
	if err := gob.NewEncoder(buf).Encode(pb); err != nil {
		panic("encode vote")
	}
	return buf.Bytes()
}

type CanonicalProposal struct {
	Height   int64
	Round    int32
	POLRound int32
	BlockID  BlockID
	Block    *Block
}
