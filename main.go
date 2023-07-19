package main

import (
	"fmt"
	"log"

	"github.com/hallazzang/tinybft/types"
)

func main() {
	config := DefaultConfig

	privVal1 := types.NewPrivValidator()
	privVal2 := types.NewPrivValidator()
	privVal3 := types.NewPrivValidator()

	valPubKey1, err := privVal1.GetPubKey()
	if err != nil {
		panic(err)
	}
	valPubKey2, err := privVal2.GetPubKey()
	if err != nil {
		panic(err)
	}
	valPubKey3, err := privVal3.GetPubKey()
	if err != nil {
		panic(err)
	}

	val1 := &types.Validator{
		Address:     valPubKey1.Address(),
		PubKey:      valPubKey1,
		VotingPower: 10000,
	}
	val2 := &types.Validator{
		Address:     valPubKey2.Address(),
		PubKey:      valPubKey2,
		VotingPower: 10000,
	}
	val3 := &types.Validator{
		Address:     valPubKey3.Address(),
		PubKey:      valPubKey3,
		VotingPower: 10000,
	}
	fmt.Println("val1 address", val1.Address)
	fmt.Println("val2 address", val2.Address)
	fmt.Println("val3 address", val3.Address)

	valSet := types.NewValidatorSet(val1)
	state := ChainState{
		InitialHeight:   1,
		LastBlockHeight: 0,
		Validators:      valSet,
	}
	blockExec := NewBlockExecutor()

	incomingMsgQueue := make(chan types.Message, 1)
	outgoingMsgQueue := make(chan types.Message, 1)
	machine1 := NewMachine(config, state, blockExec, incomingMsgQueue, outgoingMsgQueue)
	machine1.SetPrivValidator(privVal1)

	go func() {
		for msg := range outgoingMsgQueue {
			log.Printf("debug: received outgoing msg: %+v", msg)
		}
	}()
	machine1.Start()
	machine1.Wait()
}
