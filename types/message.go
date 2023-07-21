package types

import (
	"bytes"
	"encoding/gob"
)

var (
	_ Message = ProposalMessage{}
	_ Message = VoteMessage{}
)

func init() {
	gob.Register((*Message)(nil))
	gob.Register(ProposalMessage{})
	gob.Register(VoteMessage{})
}

type Message interface{}

func MarshalMessage(msg Message) ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := gob.NewEncoder(buf).Encode(&msg); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func UnmarshalMessage(b []byte) (Message, error) {
	var msg Message
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&msg); err != nil {
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
