package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProposalSignature(t *testing.T) {
	block := &Block{
		BlockHeader: BlockHeader{
			ID:   BlockID{1, 2, 3},
			Time: time.Date(2023, 7, 20, 0, 0, 0, 0, time.UTC),
		},
	}
	proposal := NewProposal(1, 0, -1, block.ID, block)

	privVal := NewPrivValidator()
	privVal.SignProposal(proposal)

	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)
	require.True(t, pubKey.VerifySignature(proposal.SignBytes(), proposal.Signature))
}
