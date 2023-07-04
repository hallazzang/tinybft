package types

import (
	"bytes"
	"errors"
	"fmt"
)

type VoteSet struct {
	height int64
	round  int32
	typ    VoteType
	valSet *ValidatorSet

	votesBitArray *BitArray
	votes         []*Vote
	sum           int64
	maj23         *BlockID
	votesByBlock  map[string]*blockVotes
}

func (vs *VoteSet) getVote(valIdx int32, blockKey string) (vote *Vote, ok bool) {
	if existing := vs.votes[valIdx]; existing != nil && existing.BlockID.Key() == blockKey {
		return existing, true
	}
	if existing := vs.votesByBlock[blockKey].getByIndex(valIdx); existing != nil {
		return existing, true
	}
	return nil, false
}

func (vs *VoteSet) AddVote(vote *Vote) (added bool, err error) {
	if vs == nil {
		panic("AddVote() on nil VoteSet")
	}
	return vs.addVote(vote)
}

func (vs *VoteSet) addVote(vote *Vote) (added bool, err error) {
	if vote == nil {
		return false, errors.New("nil vote")
	}
	valIdx := vote.ValidatorIndex
	valAddr := vote.ValidatorAddress
	blockKey := vote.BlockID.Key()
	if valIdx < 0 {
		return false, errors.New("val idx < 0")
	} else if valAddr.Empty() {
		return false, errors.New("empty address")
	}
	if (vote.Height != vs.height) ||
		(vote.Round != vs.round) ||
		(vote.Type != vs.typ) {
		return false, fmt.Errorf("expected %d/%d/%d; got %d/%d/%d\n",
			vs.height, vs.round, vs.typ, vote.Height, vote.Round, vote.Type)
	}
	lookupAddr, val := vs.valSet.GetByIndex(valIdx)
	if val == nil {
		return false, fmt.Errorf("cannot find validator %d", valIdx)
	}
	if !lookupAddr.Equal(valAddr) {
		return false, fmt.Errorf("%v != %v", lookupAddr, valAddr)
	}
	if existing, ok := vs.getVote(valIdx, blockKey); ok {
		if bytes.Equal(existing.Signature, vote.Signature) {
			return false, nil // same vote
		}
		return false, errors.New("different vote from validator")
	}
	if err := vote.Verify(val.PubKey); err != nil {
		return false, fmt.Errorf("verify vote: %w", err)
	}
	added, conflicting := vs.addVerifiedVote(vote, blockKey, val.VotingPower)
	if conflicting != nil {
		return added, errors.New("conflicting vote")
	}
	if !added {
		panic("...")
	}
	return added, nil
}

func (vs *VoteSet) addVerifiedVote(vote *Vote, blockKey string, votingPower int64) (added bool, conflicting *Vote) {
	valIdx := vote.ValidatorIndex
	if existing := vs.votes[valIdx]; existing != nil {
		if existing.BlockID == vote.BlockID {
			panic("duplicate vote")
		} else {
			conflicting = existing
		}
		if vs.maj23 != nil && vs.maj23.Key() == blockKey {
			vs.votes[valIdx] = vote
			vs.votesBitArray.SetIndex(int(valIdx), true)
		}
	} else {
		vs.votes[valIdx] = vote
		vs.votesBitArray.SetIndex(int(valIdx), true)
		vs.sum += votingPower
	}
	votesByBlock, ok := vs.votesByBlock[blockKey]
	if ok {
		if conflicting != nil && !votesByBlock.peerMaj23 {
			return false, conflicting
		}
	} else {
		if conflicting != nil {
			return false, conflicting
		}
		votesByBlock = newBlockVotes(false, vs.valSet.Size())
		vs.votesByBlock[blockKey] = votesByBlock
	}

	origSum := votesByBlock.sum
	quorum := vs.valSet.TotalVotingPower()*2/3 + 1

	votesByBlock.addVerifiedVote(vote, votingPower)
	if origSum < quorum && quorum <= votesByBlock.sum {
		if vs.maj23 == nil {
			maj23BlockID := vote.BlockID
			vs.maj23 = &maj23BlockID
			for i, vote := range votesByBlock.votes {
				if vote != nil {
					vs.votes[i] = vote
				}
			}
		}
	}
	return true, conflicting
}

func (vs *VoteSet) HasTwoThirdsAny() bool {
	if vs == nil {
		return false
	}
	return vs.sum > vs.valSet.TotalVotingPower()*2/3
}

func (vs *VoteSet) TwoThirdsMajority() (blockID BlockID, ok bool) {
	if vs == nil {
		return BlockID{}, false
	}
	if vs.maj23 != nil {
		return *vs.maj23, true
	}
	return BlockID{}, false
}

func (vs *VoteSet) HasTwoThirdsMajority() bool {
	if vs == nil {
		return false
	}
	return vs.maj23 != nil
}

type blockVotes struct {
	peerMaj23 bool
	bitArray  *BitArray
	votes     []*Vote
	sum       int64
}

func newBlockVotes(peerMaj23 bool, numVals int) *blockVotes {
	return &blockVotes{
		peerMaj23: peerMaj23,
		bitArray:  NewBitArray(numVals),
		votes:     make([]*Vote, numVals),
		sum:       0,
	}
}

func (vs *blockVotes) addVerifiedVote(vote *Vote, votingPower int64) {
	valIdx := vote.ValidatorIndex
	if existing := vs.votes[valIdx]; existing == nil {
		vs.bitArray.SetIndex(int(valIdx), true)
		vs.votes[valIdx] = vote
		vs.sum += votingPower
	}
}

func (vs *blockVotes) getByIndex(i int32) *Vote {
	if vs == nil {
		return nil
	}
	return vs.votes[i]
}

type HeightVoteSet struct{}

func NewHeightVoteSet(height int64, vs *ValidatorSet) *HeightVoteSet {
	panic("not implemented")
}

func (hvs *HeightVoteSet) SetRound(round int32) {
	panic("not implemented")
}

func (hvs *HeightVoteSet) AddVote(vote *Vote) (added bool, err error) {
	panic("not implemented")
}

func (hvs *HeightVoteSet) Prevotes(round int32) *VoteSet {
	panic("not implemented")
}

func (hvs *HeightVoteSet) Precommits(round int32) *VoteSet {
	panic("not implemented")
}

func (hvs *HeightVoteSet) POLInfo() (polRound int32, polBlockID BlockID) {
	panic("not implemented")
}

func SignAndCheckVote(vote *Vote, privVal *PrivValidator) (bool, error) {
	if err := privVal.SignVote(vote); err != nil {
		return true, err // true = recoverable
	}
	return true, nil
}
