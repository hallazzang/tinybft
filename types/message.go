package types

import (
	"encoding/json"
)

var (
	_ Message = ProposalMessage{}
	_ Message = VoteMessage{}
)

type Message interface{}

func MarshalMessage(msg Message) ([]byte, error) {
	return json.Marshal(msg)
}

func UnmarshalMessage(b []byte) (Message, error) {
	var msg Message
	if err := json.Unmarshal(b, &msg); err != nil {
		return nil, err
	}
	return msg, nil
}

type ProposalMessage struct {
	Proposal *Proposal
}

type VoteMessage struct {
	Vote *Vote
}
