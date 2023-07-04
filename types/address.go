package types

import (
	"bytes"
)

type Address []byte

func (addr Address) Empty() bool {
	return len(addr) == 0
}

func (addr Address) Equal(other Address) bool {
	return bytes.Equal(addr, other)
}
