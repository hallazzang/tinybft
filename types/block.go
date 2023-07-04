package types

import (
	"fmt"
	"time"
)

type BlockID [32]byte

func (blockID BlockID) Empty() bool {
	return blockID == [32]byte{}
}

func (blockID BlockID) Key() string {
	return fmt.Sprintf("%X", blockID)
}

type Block struct {
	BlockHeader
}

type BlockHeader struct {
	ID   BlockID
	Time time.Time
}
