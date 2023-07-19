package main

import (
	"time"

	"github.com/hallazzang/tinybft/types"
)

type BlockExecutor struct {
	blockCounter int
}

func NewBlockExecutor() *BlockExecutor {
	return &BlockExecutor{
		blockCounter: 0,
	}
}

func (blockExec *BlockExecutor) CreateProposalBlock(
	height int64, state ChainState, proposerAddr types.Address) (*types.Block, error) {
	blockExec.blockCounter++
	block := &types.Block{
		BlockHeader: types.BlockHeader{
			ID:   types.BlockID{byte(blockExec.blockCounter)},
			Time: time.Now(), // TODO: use median time from last commit
		},
	}
	return block, nil
}
