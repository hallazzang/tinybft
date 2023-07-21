package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/hallazzang/tinybft/types"
)

// Machine is the state machine.
type Machine struct {
	config        Config
	privValidator *types.PrivValidator

	blockExec *BlockExecutor

	State
	state               ChainState
	privValidatorPubKey types.PubKey

	incomingMsgQueue <-chan types.Message
	outgoingMsgQueue chan<- types.Message
	internalMsgQueue chan types.Message
	ticker           *Ticker

	quit chan struct{}
	done chan struct{}

	logger zerolog.Logger
}

// OK
func NewMachine(
	config Config,
	state ChainState,
	blockExec *BlockExecutor,
	incomingMsgQueue <-chan types.Message,
	outgoingMsgQueue chan<- types.Message) *Machine {
	m := &Machine{
		config:           config,
		state:            state,
		blockExec:        blockExec,
		incomingMsgQueue: incomingMsgQueue,
		outgoingMsgQueue: outgoingMsgQueue,
		internalMsgQueue: make(chan types.Message, 1),
		ticker:           NewTicker(),
		quit:             make(chan struct{}),
		done:             make(chan struct{}),
		logger:           zerolog.Nop(),
	}
	if state.LastBlockHeight > 0 {
		panic("reconstructing last commit is not implemented")
	}
	m.updateToState(state)
	return m
}

func (m *Machine) SetPrivValidator(priv *types.PrivValidator) {
	m.privValidator = priv
	if err := m.updatePrivValidatorPubKey(); err != nil {
		m.logger.Error().Err(err).Msg("failed to get priv validator pubkey")
	}
}

func (m *Machine) updatePrivValidatorPubKey() error {
	if m.privValidator == nil {
		return nil
	}
	pubKey, err := m.privValidator.GetPubKey()
	if err != nil {
		return err
	}
	m.privValidatorPubKey = pubKey
	return nil
}

func (m *Machine) SetLogger(logger zerolog.Logger) {
	m.logger = logger
}

func (m *Machine) Start() {
	m.StartTime = time.Now()
	go m.run()
	m.scheduleRound0()
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
			m.logger.Debug().Msg("ignoring updateToState()")
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
	m.ProposalBlock = proposal.Block // originally done in addProposalBlockPart
	m.logger.Info().Msg("received proposal")
	return nil
}

func (m *Machine) handleMsg(msg types.Message) {
	var err error
	switch msg := msg.(type) {
	case types.ProposalMessage:
		err = m.setProposal(msg.Proposal)
	// TODO: handle BlockPartMessage?
	case types.VoteMessage:
		_, err = m.tryAddVote(msg.Vote)
	default:
		panic(fmt.Sprintf("invalid message type: %T", msg))
	}
	if err != nil {
		m.logger.Error().Err(err).Msg("failed to handle message")
	}
}

func (m *Machine) handleTimeout(hrs HRS) {
	if hrs.Height != m.Height ||
		hrs.Round < m.Round ||
		(hrs.Round == m.Round && hrs.Step < m.Step) {
		m.logger.Debug().Msg("ignoring timeout because we are ahead")
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
		m.logger.Error().
			Int64("m.Height", m.Height).Int64("height", height).
			Int32("round", round).Int32("m.Round", m.Round).
			Str("m.Step", m.Step.String()).Msg(".")
		m.logger.Error().Msg("entering new round with invalid args")
		return
	}
	m.logger.Info().Msgf("entering new round: %d/%d", height, round)
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
		m.logger.Error().Msg("entering propose step with invalid args")
		return
	}
	m.logger.Info().Msg("entering propose step")
	defer func() {
		m.updateRoundStep(round, RoundStepPropose)
		m.newStep()
		if m.isProposalComplete() {
			m.enterPrevote(height, m.Round)
		}
	}()
	m.ticker.SetTimeout(HRS{height, round, RoundStepPropose}, m.config.GetProposeTimeout(round))
	if m.privValidatorPubKey == nil {
		m.logger.Error().Msg("propose step; empty priv validator public key")
		return
	}
	address := m.privValidatorPubKey.Address()
	if m.isProposer(address) {
		m.logger.Info().Msg("our turn to propose")
		m.decideProposal(height, round)
	} else {
		m.logger.Debug().Msg("not our turn to propose")
	}
}

func (m *Machine) enterPrevote(height int64, round int32) {
	if m.Height != height ||
		round < m.Round ||
		(m.Round == round && RoundStepPrevote <= m.Step) {
		m.logger.Error().Msg("entering prevote step with invalid args")
		return
	}
	m.logger.Info().Msg("entering prevote step")
	defer func() {
		m.updateRoundStep(round, RoundStepPrevote)
		m.newStep()
	}()
	// doPrevote
	if m.LockedBlock != nil {
		m.logger.Debug().Msg("prevote; already locked; prevoting locked block")
		m.signAddVote(types.Prevote, m.LockedBlock.ID)
		return
	}
	if m.ProposalBlock == nil {
		m.logger.Debug().Msg("prevote; ProposalBlock is nil")
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
		m.logger.Error().Msg("entering prevote wait step with invalid args")
		return
	}
	if !m.Votes.Prevotes(round).HasTwoThirdsAny() {
		panic(fmt.Sprintf(
			"entering prevote wait step (%d/%d), but prevotes does not have any +2/3 votes",
			height, round))
	}
	m.logger.Info().Msg("entering prevote wait step")
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
		m.logger.Error().Msg("entering precommit step with invalid args")
		return
	}
	m.logger.Info().Msg("entering precommit step")
	defer func() {
		m.updateRoundStep(round, RoundStepPrecommit)
		m.newStep()
	}()
	blockID, ok := m.Votes.Prevotes(round).TwoThirdsMajority()
	if !ok {
		if m.LockedBlock != nil {
			m.logger.Debug().Msg("precommit; no +2/3 prevotes while we are locked; precommiting nil")
		} else {
			m.logger.Debug().Msg("precommit; no +2/3 prevotes; precommiting nil")
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
			m.logger.Debug().Msg("precommit; +2/3 prevoted for nil")
		} else {
			m.logger.Debug().Msg("precommit; +2/3 prevoted for nil; unlocking")
			m.Unlock()
		}
		m.signAddVote(types.Precommit, types.BlockID{})
		return
	}
	if m.LockedBlock != nil && blockID == m.LockedBlock.ID {
		m.logger.Debug().Msg("precommit; +2/3 prevoted locked block; relocking")
		m.LockedRound = round
		m.signAddVote(types.Precommit, blockID)
		return
	}
	if blockID == m.ProposalBlock.ID {
		m.logger.Debug().Msg("precommit; +2/3 prevoted proposal block; locking")
		// TODO: validate block
		m.LockedRound = round
		m.LockedBlock = m.ProposalBlock
		m.signAddVote(types.Precommit, blockID)
		return
	}
	m.logger.Debug().Msg("precommit; +2/3 prevotes for a block we don't have; voting nil")
	m.Unlock()
	m.signAddVote(types.Precommit, types.BlockID{})
}

func (m *Machine) enterPrecommitWait(height int64, round int32) {
	if m.Height != height ||
		round < m.Round ||
		(m.Round == round && m.TriggeredPrecommitTimeout) {
		m.logger.Error().Msg("entering precommit step with invalid args")
		return
	}
	if !m.Votes.Precommits(round).HasTwoThirdsAny() {
		panic(fmt.Sprintf(
			"entering precommit wait step (%d/%d), but precommits does not have any +2/3 votes",
			height, round))
	}
	m.logger.Info().Msg("entering precommit wait step")
	defer func() {
		m.TriggeredPrecommitTimeout = true
		m.newStep()
	}()
	m.ticker.SetTimeout(HRS{height, round, RoundStepPrecommitWait}, m.config.GetPrecommitTimeout(round))
}

func (m *Machine) enterCommit(height int64, commitRound int32) {
	if m.Height != height || RoundStepCommit <= m.Step {
		m.logger.Error().Msg("entering precommit step with invalid args")
		return
	}
	m.logger.Info().Msg("entering commit step")
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
	if m.LockedBlock != nil && blockID == m.LockedBlock.ID {
		m.logger.Debug().Msg("commit is for a locked block; set ProposalBlock=LockedBlock")
		m.ProposalBlock = m.LockedBlock
	}
	if m.ProposalBlock == nil || m.ProposalBlock.ID != blockID {
		m.logger.Info().Msg("commit is for a block we don't know about; set ProposalBlock=nil")
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
		m.logger.Error().Msg("failed attempt to finalize commit; there was no +2/3 majority or +2/3 was for nil")
		return
	}
	if m.ProposalBlock == nil || m.ProposalBlock.ID != blockID {
		m.logger.Debug().Msg("failed attempt to finalize commit; we do not have the commit block")
		return
	}
	m.finalizeCommit(height)
}

func (m *Machine) finalizeCommit(height int64) {
	if m.Height != height || m.Step != RoundStepCommit {
		m.logger.Error().Msg("entering finalize commit step with invalid args")
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
	m.logger.Info().Any("block_id", blockID).Msg("commited block")
	// ApplyBlock()
	stateCopy := m.state
	stateCopy.LastBlockHeight++
	m.updateToState(stateCopy)
	m.scheduleRound0()
}

func (m *Machine) tryAddVote(vote *types.Vote) (added bool, err error) {
	return m.addVote(vote)
}

func (m *Machine) addVote(vote *types.Vote) (added bool, err error) {
	if vote.Height+1 == m.Height && vote.Type == types.Precommit {
		if m.Step != RoundStepNewHeight {
			m.logger.Debug().Any("vote", vote).Msg("ignoring vote")
			return false, nil
		}
		added, err = m.LastCommit.AddVote(vote)
		if !added {
			return false, err
		}
		m.logger.Debug().Any("vote", vote).Msg("added vote to last commit")
		// TODO: broadcast vote
		m.outgoingMsgQueue <- types.VoteMessage{Vote: vote}
		if m.config.SkipCommitTimeout && m.LastCommit.HasAll() {
			m.enterNewRound(m.Height, 0)
		}
		return added, err
	}
	if vote.Height != m.Height {
		m.logger.Debug().Msgf("vote ignored: %d < %d", vote.Height, m.Height)
		return false, nil
	}
	added, err = m.Votes.AddVote(vote)
	if !added {
		return false, err
	}
	// TODO: broadcast vote
	m.outgoingMsgQueue <- types.VoteMessage{Vote: vote}

	switch vote.Type {
	case types.Prevote:
		prevotes := m.Votes.Prevotes(vote.Round)
		m.logger.Debug().Any("vote", vote).Msg("added vote to prevote")
		if blockID, ok := prevotes.TwoThirdsMajority(); ok {
			if (m.LockedBlock != nil) &&
				(m.LockedRound < vote.Round) &&
				(vote.Round <= m.Round) &&
				m.LockedBlock.ID != blockID {
				m.logger.Debug().Msg("unlocking because of PoL")
				m.Unlock()
			}

			if !blockID.Empty() &&
				(m.ValidRound < vote.Round) &&
				(vote.Round == m.Round) {
				if m.ProposalBlock.ID == blockID {
					m.logger.Debug().Msg("updating valid block because of POL")
					m.ValidRound = vote.Round
					m.ValidBlock = m.ProposalBlock
				} else {
					m.logger.Debug().Msg("valid block we do not know about; set ProposalBlock=nil")
					m.ProposalBlock = nil
				}
			} else {
				m.logger.Debug().Msg("valid block we do not know about")
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
		m.logger.Debug().Any("vote", vote).Msg("added vote to precommit")
		blockID, ok := precommits.TwoThirdsMajority()
		if ok {
			m.enterNewRound(m.Height, vote.Round)
			m.enterPrecommit(m.Height, vote.Round)

			if !blockID.Empty() {
				m.enterCommit(m.Height, vote.Round)
				if m.config.SkipCommitTimeout && precommits.HasAll() {
					m.enterNewRound(m.Height, 0)
				}
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
	if m.privValidator == nil {
		return
	}
	if m.privValidatorPubKey == nil {
		return
	}
	if !m.Validators.HasAddress(m.privValidatorPubKey.Address()) {
		return
	}
	vote, err := m.signVote(typ, blockID)
	if err != nil {
		m.logger.Error().Err(err).Msg("failed to sign vote")
		return
	}
	m.sendInternalMsg(types.VoteMessage{Vote: vote})
	m.logger.Info().Msg("signed and pushed vote")
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
			m.logger.Error().Err(err).Msg("unable to create proposal block: %v")
			return
		} else if block == nil {
			panic("Method createProposalBlock should not provide a nil block without errors")
		}
	}
	proposal := types.NewProposal(height, round, m.ValidRound, block.ID, block)
	if err := m.privValidator.SignProposal(proposal); err == nil {
		m.sendInternalMsg(types.ProposalMessage{Proposal: proposal})
		m.logger.Debug().Msg("signed proposal")
	} else {
		m.logger.Error().Err(err).Msg("propose step; failed signing proposal")
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
	proposerAddr := m.privValidatorPubKey.Address()
	block, err := m.blockExec.CreateProposalBlock(m.Height, m.state, proposerAddr)
	if err != nil {
		panic(err)
	}
	return block, nil
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
