package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBitArray(t *testing.T) {
	ba := NewBitArray(10)
	require.False(t, ba.GetIndex(5))
	ba.SetIndex(5, true)
	require.True(t, ba.GetIndex(5))
	ba.SetIndex(5, false)
	require.False(t, ba.GetIndex(5))
}
