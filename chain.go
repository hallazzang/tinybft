package main

import (
	"github.com/hallazzang/tinybft/types"
)

type ChainState struct {
	InitialHeight int64

	LastBlockHeight int64

	Validators *types.ValidatorSet
}
