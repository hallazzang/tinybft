package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTicker(t *testing.T) {
	ticker := NewTicker()
	ticker.SetTimeout(HRS{1, 1, 1}, 100*time.Millisecond)
	select {
	case <-ticker.C:
		t.FailNow()
	default:
	}
	time.Sleep(50 * time.Millisecond)
	ticker.SetTimeout(HRS{1, 1, 2}, 100*time.Millisecond)
	time.Sleep(55 * time.Millisecond)
	select {
	case <-ticker.C:
		t.FailNow()
	default:
	}
	time.Sleep(50 * time.Millisecond)
	select {
	case hrs := <-ticker.C:
		require.Equal(t, HRS{1, 1, 2}, hrs)
	default:
		t.FailNow()
	}
}
