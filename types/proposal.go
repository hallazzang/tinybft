package types

import (
	"encoding/json"
	"fmt"
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
	bz, err := json.Marshal(pb)
	if err != nil {
		panic(fmt.Errorf("marshal proposal: %w", err))
	}
	return bz
}

type CanonicalProposal struct {
	Height   int64
	Round    int32
	POLRound int32
	BlockID  BlockID
	Block    *Block
}
