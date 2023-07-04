package main

import (
	"github.com/hallazzang/tinybft/types"
)

func main() {
	config := DefaultConfig

	val1 := &types.Validator{
		Address:     types.Address{},
		PubKey:      nil,
		VotingPower: 0,
	}
	val2 := &types.Validator{
		Address:     types.Address{},
		PubKey:      nil,
		VotingPower: 0,
	}
	valSet := types.NewValidatorSet(val1, val2)
	state := ChainState{
		InitialHeight:   1,
		LastBlockHeight: 0,
		Validators:      valSet,
	}

	incomingMsgQueue := make(chan types.Message, 1)
	outgoingMsgQueue := make(chan types.Message, 1)
	machine1 := NewMachine(config, state, incomingMsgQueue, outgoingMsgQueue)

	machine1.Start()
	machine1.Wait()
}
