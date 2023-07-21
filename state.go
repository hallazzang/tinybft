package main

import (
	"fmt"
	"time"

	"github.com/hallazzang/tinybft/types"
)

type RoundStep int

const (
	RoundStepNewHeight     = RoundStep(1)
	RoundStepNewRound      = RoundStep(2)
	RoundStepPropose       = RoundStep(3)
	RoundStepPrevote       = RoundStep(4)
	RoundStepPrevoteWait   = RoundStep(5)
	RoundStepPrecommit     = RoundStep(6)
	RoundStepPrecommitWait = RoundStep(7)
	RoundStepCommit        = RoundStep(8)
)

func (step RoundStep) String() string {
	switch step {
	case RoundStepNewHeight:
		return "RoundStepNewHeight"
	case RoundStepNewRound:
		return "RoundStepNewRound"
	case RoundStepPropose:
		return "RoundStepPropose"
	case RoundStepPrevote:
		return "RoundStepPrevote"
	case RoundStepPrevoteWait:
		return "RoundStepPrevoteWait"
	case RoundStepPrecommit:
		return "RoundStepPrecommit"
	case RoundStepPrecommitWait:
		return "RoundStepPrecommitWait"
	case RoundStepCommit:
		return "RoundStepCommit"
	default:
		return fmt.Sprintf("RoundStep(%d)", step)
	}
}

type State struct {
	HRS

	StartTime time.Time

	CommitTime    time.Time
	Validators    *types.ValidatorSet
	Proposal      *types.Proposal
	ProposalBlock *types.Block
	LockedRound   int32
	LockedBlock   *types.Block

	ValidRound int32
	ValidBlock *types.Block

	Votes                     *types.HeightVoteSet
	CommitRound               int32
	LastCommit                *types.VoteSet
	TriggeredPrecommitTimeout bool
}

func (state *State) Empty() bool {
	return state.Validators == nil
}

func (state *State) Unlock() {
	state.LockedRound = -1
	state.LockedBlock = nil
}

type HRS struct {
	Height int64
	Round  int32
	Step   RoundStep
}

func (hrs HRS) Cmp(other HRS) int {
	if hrs.Height < other.Height {
		return -1
	} else if hrs.Height > other.Height {
		return 1
	}
	if hrs.Round < other.Round {
		return -1
	} else if hrs.Round > other.Round {
		return 1
	}
	if hrs.Step < other.Step {
		return -1
	} else if hrs.Step > other.Step {
		return 1
	}
	return 0
}
