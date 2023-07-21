package main

import (
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/hallazzang/tinybft/types"
)

func newValidator(votingPower int64) (*types.PrivValidator, *types.Validator) {
	privVal := types.NewPrivValidator()
	pubKey, err := privVal.GetPubKey()
	if err != nil {
		panic(err)
	}
	return privVal, types.NewValidator(pubKey, votingPower)
}

func main() {
	config := DefaultConfig
	config.ProposeTimeout = 500 * time.Millisecond
	config.PrevoteTimeout = 500 * time.Millisecond
	config.PrecommitTimeout = 500 * time.Millisecond
	config.CommitTimeout = 500 * time.Millisecond

	privVal1, val1 := newValidator(10000)
	privVal2, val2 := newValidator(10000)
	privVal3, val3 := newValidator(10000)
	privVal4, val4 := newValidator(10000)
	blockExec := NewBlockExecutor()

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	bus := types.NewMessageBus(redisClient, "tinybft:message")

	pubCh := make(chan types.Message, 100)
	go func() {
		for msg := range pubCh {
			if err := bus.Publish(msg); err != nil {
				panic(err)
			}
		}
	}()

	vals := []*types.Validator{val1, val2, val3, val4}
	privVals := []*types.PrivValidator{privVal1, privVal2, privVal3, privVal4}

	valSet := types.NewValidatorSet(vals...)
	state := ChainState{
		InitialHeight:   1,
		LastBlockHeight: 0,
		Validators:      valSet,
	}
	var machines []*Machine
	for i := range vals {
		ch, closeCh := bus.Subscribe()
		defer closeCh()
		m := NewMachine(config, state, blockExec, ch, pubCh)
		m.SetPrivValidator(privVals[i])
		if i == 1 {
			m.SetLogger(log.Output(zerolog.ConsoleWriter{Out: os.Stderr}))
		}
		m.Start()
		machines = append(machines, m)
	}

	for _, m := range machines {
		m.Wait()
	}
}
