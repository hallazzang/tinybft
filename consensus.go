package main

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/hallazzang/tinybft/types"
)

// Machine is the state machine.
type Machine struct {
	config        Config
	privValidator *types.PrivValidator

	State
	state               ChainState
	privValidatorPubKey types.PubKey

	incomingMsgQueue <-chan types.Message
	outgoingMsgQueue chan<- types.Message
	internalMsgQueue chan types.Message
	ticker           *Ticker

	quit chan struct{}
	done chan struct{}
}

// OK
func NewMachine(
	config Config,
	state ChainState,
	incomingMsgQueue <-chan types.Message,
	outgoingMsgQueue chan<- types.Message) *Machine {
	m := &Machine{
		config:           config,
		state:            state,
		incomingMsgQueue: incomingMsgQueue,
		outgoingMsgQueue: outgoingMsgQueue,
		internalMsgQueue: make(chan types.Message, 1),
		ticker:           NewTicker(),
		quit:             make(chan struct{}),
		done:             make(chan struct{}),
	}
	if state.LastBlockHeight > 0 {
		panic("reconstructing last commit is not implemented")
	}
	m.updateToState(state)
	return m
}

func (m *Machine) Start() {
	m.StartTime = time.Now()
	go m.run()
}

func (m *Machine) Stop() {
	m.ticker.Stop()
	close(m.quit)
}

func (m *Machine) Wait() {
	<-m.done
}

// OK
func (m *Machine) sendInternalMsg(msg types.Message) {
	m.internalMsgQueue <- msg
}

// OK
func (m *Machine) updateToState(state ChainState) {
	if m.CommitRound > -1 && 0 < m.Height && m.Height != state.LastBlockHeight {
		panic(fmt.Sprintf("expected height %d, got %d", m.Height, state.LastBlockHeight))
	}

	if !m.State.Empty() {
		if m.state.LastBlockHeight > 0 && m.state.LastBlockHeight+1 != m.Height {
			panic(fmt.Sprintf("%d vs %d", m.state.LastBlockHeight+1, m.Height))
		}
		if m.state.LastBlockHeight > 0 && m.Height == m.state.InitialHeight {
			panic(fmt.Sprintf("..."))
		}
		if state.LastBlockHeight <= m.state.LastBlockHeight {
			log.Printf("debug: ignoring updateToState()")
			m.newStep()
			return
		}
	}

	switch {
	case state.LastBlockHeight == 0:
		m.LastCommit = nil
	case m.CommitRound > -1 && m.Votes != nil:
		if !m.Votes.Precommits(m.CommitRound).HasTwoThirdsMajority() {
			panic("...")
		}
		m.LastCommit = m.Votes.Precommits(m.CommitRound)
	case m.LastCommit == nil:
		panic(fmt.Sprintf(
			"last commit cannot be empty after initial block (H:%d)",
			state.LastBlockHeight+1,
		))
	}

	height := state.LastBlockHeight + 1
	if height == 1 {
		height = state.InitialHeight
	}
	m.updateHeight(height)
	m.updateRoundStep(0, RoundStepNewHeight)

	if m.CommitTime.IsZero() {
		m.StartTime = time.Now().Add(m.config.CommitTimeout)
	} else {
		m.StartTime = m.CommitTime.Add(m.config.CommitTimeout)
	}

	validators := state.Validators
	m.Validators = validators
	m.Proposal = nil
	m.ProposalBlock = nil
	m.LockedRound = -1
	m.LockedBlock = nil
	m.ValidRound = -1
	m.ValidBlock = nil
	m.Votes = types.NewHeightVoteSet(height, validators)
	m.CommitRound = -1
	m.TriggeredPrecommitTimeout = false

	m.state = state
	m.newStep()
}

// OK
func (m *Machine) scheduleRound0() {
	duration := m.StartTime.Sub(time.Now())
	m.ticker.SetTimeout(HRS{m.Height, 0, RoundStepNewHeight}, duration)
}

// OK
func (m *Machine) updateHeight(height int64) {
	m.Height = height
}

// OK
func (m *Machine) updateRoundStep(round int32, step RoundStep) {
	m.Round = round
	m.Step = step
}

func (m *Machine) newStep() {
	// TODO: broadcast new round step
}

func (m *Machine) run() {
	defer func() {
		close(m.done)
	}()
	for {
		select {
		// TODO: handleTxsAvailable()
		case msg := <-m.incomingMsgQueue:
			m.handleMsg(msg)
		case msg := <-m.internalMsgQueue:
			m.handleMsg(msg)
		case hrs := <-m.ticker.C:
			m.handleTimeout(hrs)
		case <-m.quit:
			return
		}
	}
}

// OK
func (m *Machine) setProposal(proposal *types.Proposal) error {
	if m.Proposal != nil {
		return nil
	}
	if proposal.Height != m.Height || proposal.Round != m.Round {
		return nil
	}
	if proposal.POLRound < -1 ||
		(proposal.POLRound >= 0 && proposal.POLRound >= proposal.Round) {
		return errors.New("invalid proposal PoL round")
	}
	if !m.Validators.GetProposer().PubKey.VerifySignature(
		proposal.SignBytes(), proposal.Signature) {
		return errors.New("invalid proposal signature")
	}
	m.Proposal = proposal
	log.Printf("info: received proposal")
	return nil
}

func (m *Machine) handleMsg(msg types.Message) {
	var err error
	switch msg := msg.(type) {
	case *types.ProposalMessage:
		err = m.setProposal(msg.Proposal)
	// TODO: handle BlockPartMessage?
	case *types.VoteMessage:
		_, err = m.tryAddVote(msg.Vote)
	default:
		panic(fmt.Sprintf("invalid message type: %T", msg))
	}
	if err != nil {
		log.Printf("error: failed to handle message: %v", err)
	}
}

func (m *Machine) handleTimeout(hrs HRS) {
	if hrs.Height != m.Height ||
		hrs.Round < m.Round ||
		(hrs.Round == m.Round && hrs.Step < m.Step) {
		log.Printf("debug: ignoring timeout because we are ahead")
		return
	}
	switch hrs.Step {
	case RoundStepNewHeight:
		m.enterNewRound(hrs.Height, 0)
	case RoundStepNewRound:
		m.enterPropose(hrs.Height, hrs.Round)
	case RoundStepPropose:
		m.enterPrevote(hrs.Height, hrs.Round)
	case RoundStepPrevoteWait:
		m.enterPrecommit(hrs.Height, hrs.Round)
	case RoundStepPrecommitWait:
		m.enterPrecommit(hrs.Height, hrs.Round)
		m.enterNewRound(hrs.Height, hrs.Round+1)
	default:
		panic(fmt.Sprintf("invalid timeout step: %v", hrs.Step))
	}
}

func (m *Machine) enterNewRound(height int64, round int32) {
	if m.Height != height ||
		round < m.Round ||
		(m.Round == round && m.Step != RoundStepNewHeight) {
		log.Printf("error: entering new round with invalid args")
		return
	}
	log.Printf("info: entering new round: %d/%d", height, round)
	if m.Round < round {
		// TODO: increment proposer priority
	}
	validators := m.Validators
	if m.Round < round {
		validators = validators.Copy()
		validators.IncrementProposerPriority(round - m.Round)
	}
	m.updateRoundStep(round, RoundStepNewRound)
	m.Validators = validators
	if round != 0 {
		m.Proposal = nil
		m.ProposalBlock = nil
	}
	m.Votes.SetRound(round + 1)
	m.TriggeredPrecommitTimeout = false
	// TODO: wait for txs?
	m.enterPropose(height, round)
}

func (m *Machine) enterPropose(height int64, round int32) {
	if m.Height != height ||
		round < m.Round ||
		(m.Round == round && RoundStepPropose <= m.Step) {
		log.Printf("error: entering propose step with invalid args")
		return
	}
	log.Printf("info: entering propose step")
	defer func() {
		m.updateRoundStep(round, RoundStepPropose)
		m.newStep()
		if m.isProposalComplete() {
			m.enterPrevote(height, m.Round)
		}
	}()
	m.ticker.SetTimeout(HRS{height, round, RoundStepPropose}, m.config.GetProposeTimeout(round))
	// TODO: check if we're validator
	address := types.Address{}
	if m.isProposer(address) {
		log.Printf("info: our turn to propose")
		m.decideProposal(height, round)
	} else {
		log.Printf("debug: not our turn to propose")
	}
}

func (m *Machine) enterPrevote(height int64, round int32) {
	if m.Height != height ||
		round < m.Round ||
		(m.Round == round && RoundStepPrevote <= m.Step) {
		log.Printf("error: entering prevote step with invalid args")
		return
	}
	log.Printf("info: entering prevote step")
	defer func() {
		m.updateRoundStep(round, RoundStepPrevote)
		m.newStep()
	}()
	// doPrevote
	if m.LockedBlock != nil {
		log.Printf("debug: prevote; already locked; prevoting locked block")
		m.signAddVote(types.Prevote, m.LockedBlock.ID)
		return
	}
	if m.ProposalBlock == nil {
		log.Printf("debug: prevote; ProposalBlock is nil")
		m.signAddVote(types.Prevote, types.BlockID{})
		return
	}
	// TODO: validate block?
	// ProcessProposal() here
	m.signAddVote(types.Prevote, m.ProposalBlock.ID)
}

func (m *Machine) enterPrevoteWait(height int64, round int32) {
	if m.Height != height ||
		round < m.Round ||
		(m.Round == round && RoundStepPrevoteWait <= m.Step) {
		log.Printf("error: entering prevote wait step with invalid args")
		return
	}
	if !m.Votes.Prevotes(round).HasTwoThirdsAny() {
		panic(fmt.Sprintf(
			"entering prevote wait step (%d/%d), but prevotes does not have any +2/3 votes",
			height, round))
	}
	log.Printf("info: entering prevote wait step")
	defer func() {
		m.updateRoundStep(round, RoundStepPrevoteWait)
		m.newStep()
	}()
	m.ticker.SetTimeout(HRS{height, round, RoundStepPrevoteWait}, m.config.GetPrevoteTimeout(round))
}

func (m *Machine) enterPrecommit(height int64, round int32) {
	if m.Height != height ||
		round < m.Round ||
		(m.Round == round && RoundStepPrecommit <= m.Step) {
		log.Printf("error: entering precommit step with invalid args")
		return
	}
	log.Printf("info: entering precommit step")
	defer func() {
		m.updateRoundStep(round, RoundStepPrecommit)
		m.newStep()
	}()
	blockID, ok := m.Votes.Prevotes(round).TwoThirdsMajority()
	if ok {
		if m.LockedBlock == nil {
			log.Printf("debug: precommit; no +2/3 prevotes while we are locked; precommiting nil")
		} else {
			log.Printf("debug: precommit; no +2/3 prevotes; precommiting nil")
		}
		m.signAddVote(types.Precommit, types.BlockID{})
		return
	}
	polRound, _ := m.Votes.POLInfo()
	if polRound < round {
		panic(fmt.Sprintf("POLRound must be %d; got %d", round, polRound))
	}
	if blockID.Empty() {
		if m.LockedBlock == nil {
			log.Printf("debug: precommit; +2/3 prevoted for nil")
		} else {
			log.Printf("debug: precommit; +2/3 prevoted for nil; unlocking")
			m.Unlock()
		}
		m.signAddVote(types.Precommit, types.BlockID{})
		return
	}
	if blockID == m.LockedBlock.ID {
		log.Printf("debug: precommit; +2/3 prevoted locked block; relocking")
		m.LockedRound = round
		m.signAddVote(types.Precommit, blockID)
		return
	}
	if blockID == m.ProposalBlock.ID {
		log.Printf("debug: precommit; +2/3 prevoted proposal block; locking")
		// TODO: validate block
		m.LockedRound = round
		m.LockedBlock = m.ProposalBlock
		m.signAddVote(types.Precommit, blockID)
		return
	}
	log.Printf("debug: precommit; +2/3 prevotes for a block we don't have; voting nil")
	m.Unlock()
	m.signAddVote(types.Precommit, types.BlockID{})
}

func (m *Machine) enterPrecommitWait(height int64, round int32) {
	if m.Height != height ||
		round < m.Round ||
		(m.Round == round && m.TriggeredPrecommitTimeout) {
		log.Printf("error: entering precommit step with invalid args")
		return
	}
	if !m.Votes.Precommits(round).HasTwoThirdsAny() {
		panic(fmt.Sprintf(
			"entering precommit wait step (%d/%d), but precommits does not have any +2/3 votes",
			height, round))
	}
	log.Printf("info: entering precommit wait step")
	defer func() {
		m.TriggeredPrecommitTimeout = true
		m.newStep()
	}()
	m.ticker.SetTimeout(HRS{height, round, RoundStepPrecommitWait}, m.config.GetPrecommitTimeout(round))
}

func (m *Machine) enterCommit(height int64, commitRound int32) {
	if m.Height != height || RoundStepCommit <= m.Step {
		log.Printf("error: entering precommit step with invalid args")
		return
	}
	log.Printf("info: entering commit step")
	defer func() {
		m.updateRoundStep(m.Round, RoundStepCommit)
		m.CommitRound = commitRound
		m.CommitTime = time.Now()
		m.newStep()

		m.tryFinalizeCommit(height)
	}()
	blockID, ok := m.Votes.Precommits(commitRound).TwoThirdsMajority()
	if !ok {
		panic("expected +2/3 precommits")
	}
	if blockID == m.LockedBlock.ID {
		log.Printf("debug: commit is for a locked block")
		m.ProposalBlock = m.LockedBlock
	}
	if m.ProposalBlock.ID != blockID {
		log.Printf("info: commit is for a block we don't know about; set ProposalBlock=nil")
		m.ProposalBlock = nil
		// TODO: broadcast valid block?
	}
}

func (m *Machine) tryFinalizeCommit(height int64) {
	if m.Height != height {
		panic(fmt.Sprintf("tryFinalizeCommit: %d != %d", m.Height, height))
	}
	blockID, ok := m.Votes.Precommits(m.CommitRound).TwoThirdsMajority()
	if !ok || blockID.Empty() {
		log.Printf("error: failed attempt to finalize commit; there was no +2/3 majority or +2/3 was for nil")
		return
	}
	if m.ProposalBlock.ID != blockID {
		log.Printf("debug: failed attempt to finalize commit; we do not have the commit block")
		return
	}
	m.finalizeCommit(height)
}

func (m *Machine) finalizeCommit(height int64) {
	if m.Height != height || m.Step != RoundStepCommit {
		log.Printf("error: entering finalize commit step with invalid args")
		return
	}
	blockID, ok := m.Votes.Precommits(m.CommitRound).TwoThirdsMajority()
	if !ok {
		panic("cannot finalize commit; commit does not have 2/3 majority")
	}
	block := m.ProposalBlock
	if block.ID != blockID {
		panic("cannot finalize commit; proposal block does not hash to commit hash")
	}
	// ValidateBlock()
	log.Printf("info: commited block: %v", blockID)
	// ApplyBlock()
	newState := m.state
	m.updateToState(newState)
	// TODO: updatePrivValidatorPubKey
	m.scheduleRound0()
}

func (m *Machine) tryAddVote(vote *types.Vote) (added bool, err error) {
	return m.addVote(vote)
}

func (m *Machine) addVote(vote *types.Vote) (added bool, err error) {
	if vote.Height+1 == m.Height && vote.Type == types.Precommit {
		if m.Step != RoundStepNewHeight {
			log.Printf("debug: ignoring vote: %v", vote)
			return false, nil
		}
		added, err = m.LastCommit.AddVote(vote)
		if !added {
			return false, err
		}
		log.Printf("debug: added vote to last commit: %v", vote)
		// TODO: broadcast vote
		//if m.config.SkipCommitTimeout && m.LastCommit.HasAll() {
		//	m.enterNewRound(m.Height, 0)
		//}
		return true, err
	}
	if vote.Height != m.Height {
		log.Printf("debug: vote ignored: %d < %d", vote.Height, m.Height)
		return false, nil
	}
	added, err = m.Votes.AddVote(vote)
	if !added {
		return false, err
	}
	// TODO: broadcast vote

	switch vote.Type {
	case types.Prevote:
		prevotes := m.Votes.Prevotes(vote.Round)
		log.Printf("debug: added vote to prevote: %v", vote)
		if blockID, ok := prevotes.TwoThirdsMajority(); ok {
			if (m.LockedBlock != nil) &&
				(m.LockedRound < vote.Round) &&
				(vote.Round <= m.Round) &&
				m.LockedBlock.ID != blockID {
				log.Printf("debug: unlocking because of PoL")
				m.Unlock()
			}

			if !blockID.Empty() &&
				(m.ValidRound < vote.Round) &&
				(vote.Round == m.Round) {
				m.ValidRound = vote.Round
				m.ValidBlock = m.ProposalBlock
			} else {
				log.Printf("debug: valid block we do not know about")
				m.ProposalBlock = nil
			}

			// TODO: broadcast valid block
		}

		switch {
		case m.Round < vote.Round && prevotes.HasTwoThirdsAny():
			m.enterNewRound(m.Height, vote.Round)
		case m.Round == vote.Round && RoundStepPrevote <= m.Step:
			blockID, ok := prevotes.TwoThirdsMajority()
			if ok && (m.isProposalComplete() || blockID.Empty()) {
				m.enterPrecommit(m.Height, vote.Round)
			} else if prevotes.HasTwoThirdsAny() {
				m.enterPrevoteWait(m.Height, vote.Round)
			}
		case m.Proposal != nil && 0 <= m.Proposal.POLRound && m.Proposal.POLRound == vote.Round:
			if m.isProposalComplete() {
				m.enterPrevote(m.Height, m.Round)
			}
		}
	case types.Precommit:
		precommits := m.Votes.Precommits(vote.Round)
		log.Printf("debug: added vote to precommit: %v", vote)
		blockID, ok := precommits.TwoThirdsMajority()
		if ok {
			m.enterNewRound(m.Height, vote.Round)
			m.enterPrecommit(m.Height, vote.Round)

			if !blockID.Empty() {
				m.enterCommit(m.Height, vote.Round)
				//if m.config.SkipCommitTimeout && precommits.HasAll() {
				//	m.enterNewRound(m.Height, 0)
				//}
			} else {
				m.enterPrecommitWait(m.Height, vote.Round)
			}
		} else if m.Round <= vote.Round && precommits.HasTwoThirdsAny() {
			m.enterNewRound(m.Height, vote.Round)
			m.enterPrecommitWait(m.Height, vote.Round)
		}
	default:
		panic(fmt.Sprintf("invalid vote type: %v", vote.Type))
	}
	return
}

func (m *Machine) signAddVote(typ types.VoteType, blockID types.BlockID) {
	// TODO: check if we're validator
	addr := types.Address{}
	if !m.Validators.HasAddress(addr) {
		// Node is not in the validator set
		return
	}
	vote, err := m.signVote(typ, blockID)
	if err != nil {
		log.Printf("error: failed to sign vote")
		return
	}
	m.sendInternalMsg(types.VoteMessage{Vote: vote})
	log.Printf("signed and pushed vote")
}

func (m *Machine) signVote(typ types.VoteType, blockID types.BlockID) (*types.Vote, error) {
	if m.privValidatorPubKey == nil {
		return nil, errors.New("pub key is not set")
	}
	addr := m.privValidatorPubKey.Address()
	valIdx, _ := m.Validators.GetByAddress(addr)
	vote := &types.Vote{
		Type:             typ,
		Height:           m.Height,
		Round:            m.Round,
		BlockID:          blockID,
		Timestamp:        m.voteTime(),
		ValidatorAddress: addr,
		ValidatorIndex:   valIdx,
	}
	// ExtendVote()
	recoverable, err := types.SignAndCheckVote(vote, m.privValidator)
	if err != nil && !recoverable {
		panic("...")
	}
	return vote, err
}

// OK
func (m *Machine) voteTime() time.Time {
	now := time.Now()
	minVoteTime := now
	const timeIota = time.Millisecond
	if m.LockedBlock != nil {
		minVoteTime = m.LockedBlock.Time.Add(timeIota)
	} else if m.ProposalBlock != nil {
		minVoteTime = m.ProposalBlock.Time.Add(timeIota)
	}
	if now.After(minVoteTime) {
		return now
	}
	return minVoteTime
}

func (m *Machine) decideProposal(height int64, round int32) {
	var block *types.Block
	if m.ValidBlock != nil {
		block = m.ValidBlock
	} else {
		var err error
		block, err = m.createProposalBlock()
		if err != nil {
			log.Printf("error: unable to create proposal block: %v", err)
			return
		} else if block == nil {
			panic("Method createProposalBlock should not provide a nil block without errors")
		}
	}
	propBlockID := types.BlockID{} // TODO: implement
	proposal := types.NewProposal(height, round, m.ValidRound, propBlockID)
	if err := m.privValidator.SignProposal(proposal); err == nil {
		m.sendInternalMsg(types.ProposalMessage{Proposal: proposal})
		log.Printf("debug: signed proposal")
	} else {
		log.Printf("error: propose step; failed signing proposal")
	}
}

func (m *Machine) createProposalBlock() (*types.Block, error) {
	if m.privValidator == nil {
		return nil, errors.New("entered createProposalBlock with privValidator being nil")
	}
	if m.privValidatorPubKey == nil {
		return nil, fmt.Errorf("propose step; empty priv validator public key")
	}
	// CreateProposalBlock()
	return &types.Block{}, nil // TODO: implement
}

func (m *Machine) isProposer(addr types.Address) bool {
	return m.Validators.GetProposer().Address.Equal(addr)
}

func (m *Machine) isProposalComplete() bool {
	if m.Proposal == nil || m.ProposalBlock == nil {
		return false
	}
	if m.Proposal.POLRound < 0 {
		return true
	}
	return m.Votes.Prevotes(m.Proposal.POLRound).HasTwoThirdsMajority()
}
