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

func NewVoteSet(height int64, round int32, typ VoteType, valSet *ValidatorSet) *VoteSet {
	return &VoteSet{
		height:        height,
		round:         round,
		typ:           typ,
		valSet:        valSet,
		votesBitArray: NewBitArray(valSet.Size()),
		votes:         make([]*Vote, valSet.Size()),
		sum:           0,
		maj23:         nil,
		votesByBlock:  map[string]*blockVotes{},
	}
}

func (vs *VoteSet) HasAll() bool {
	return vs.sum == vs.valSet.TotalVotingPower()
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

type RoundVoteSet struct {
	Prevotes   *VoteSet
	Precommits *VoteSet
}

type HeightVoteSet struct {
	height        int64
	valSet        *ValidatorSet
	round         int32
	roundVoteSets map[int32]*RoundVoteSet
}

func NewHeightVoteSet(height int64, valSet *ValidatorSet) *HeightVoteSet {
	hvs := &HeightVoteSet{}
	hvs.Reset(height, valSet)
	return hvs
}

func (hvs *HeightVoteSet) Reset(height int64, valSet *ValidatorSet) {
	hvs.height = height
	hvs.valSet = valSet
	hvs.roundVoteSets = map[int32]*RoundVoteSet{}
	hvs.addRound(0)
	hvs.round = 0
}

func (hvs *HeightVoteSet) SetRound(round int32) {
	newRound := hvs.round - 1
	if hvs.round != 0 && (round < newRound) {
		panic("SetRound() must increment hvs.round")
	}
	for r := newRound; r <= round; r++ {
		if _, ok := hvs.roundVoteSets[r]; ok {
			continue // Already exists because peerCatchupRounds.
		}
		hvs.addRound(r)
	}
	hvs.round = round
}

func (hvs *HeightVoteSet) AddVote(vote *Vote) (added bool, err error) {
	// TODO: validate vote type
	voteSet := hvs.getVoteSet(vote.Round, vote.Type)
	if voteSet == nil {
		// TODO: peerCatchupRounds?
		hvs.addRound(vote.Round)
		voteSet = hvs.getVoteSet(vote.Round, vote.Type)
	}
	added, err = voteSet.AddVote(vote)
	return
}

func (hvs *HeightVoteSet) Prevotes(round int32) *VoteSet {
	return hvs.getVoteSet(round, Prevote)
}

func (hvs *HeightVoteSet) Precommits(round int32) *VoteSet {
	return hvs.getVoteSet(round, Precommit)
}

func (hvs *HeightVoteSet) POLInfo() (polRound int32, polBlockID BlockID) {
	for r := hvs.round; r >= 0; r-- {
		rvs := hvs.getVoteSet(r, Prevote)
		polBlockID, ok := rvs.TwoThirdsMajority()
		if ok {
			return r, polBlockID
		}
	}
	return -1, BlockID{}
}

func (hvs *HeightVoteSet) getVoteSet(round int32, typ VoteType) *VoteSet {
	rvs, ok := hvs.roundVoteSets[round]
	if !ok {
		return nil
	}
	switch typ {
	case Prevote:
		return rvs.Prevotes
	case Precommit:
		return rvs.Precommits
	default:
		panic(fmt.Sprintf("Unexpected vote type %X", typ))
	}
}

func (hvs *HeightVoteSet) addRound(round int32) {
	if _, ok := hvs.roundVoteSets[round]; ok {
		panic("addRound() for an existing round")
	}
	prevotes := NewVoteSet(hvs.height, round, Prevote, hvs.valSet)
	precommits := NewVoteSet(hvs.height, round, Precommit, hvs.valSet)
	hvs.roundVoteSets[round] = &RoundVoteSet{
		Prevotes:   prevotes,
		Precommits: precommits,
	}
}

func SignAndCheckVote(vote *Vote, privVal *PrivValidator) (bool, error) {
	if err := privVal.SignVote(vote); err != nil {
		return true, err // true = recoverable
	}
	return true, nil
}
