package types

type Message interface{}

type ProposalMessage struct {
	Proposal *Proposal
}

type VoteMessage struct {
	Vote *Vote
}
