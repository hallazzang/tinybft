package main

import (
	"time"
)

var DefaultConfig = Config{
	ProposeTimeout:        3000 * time.Millisecond,
	ProposeTimeoutDelta:   500 * time.Millisecond,
	PrevoteTimeout:        1000 * time.Millisecond,
	PrevoteTimeoutDelta:   500 * time.Millisecond,
	PrecommitTimeout:      1000 * time.Millisecond,
	PrecommitTimeoutDelta: 500 * time.Millisecond,
	CommitTimeout:         1000 * time.Millisecond,
	SkipCommitTimeout:     false,
}

type Config struct {
	ProposeTimeout        time.Duration
	ProposeTimeoutDelta   time.Duration
	PrevoteTimeout        time.Duration
	PrevoteTimeoutDelta   time.Duration
	PrecommitTimeout      time.Duration
	PrecommitTimeoutDelta time.Duration
	CommitTimeout         time.Duration
	SkipCommitTimeout     bool
}

func (cfg Config) GetProposeTimeout(round int32) time.Duration {
	return cfg.ProposeTimeout + cfg.ProposeTimeoutDelta*time.Duration(round)
}

func (cfg Config) GetPrevoteTimeout(round int32) time.Duration {
	return cfg.PrevoteTimeout + cfg.PrevoteTimeoutDelta*time.Duration(round)
}

func (cfg Config) GetPrecommitTimeout(round int32) time.Duration {
	return cfg.PrecommitTimeout + cfg.PrecommitTimeoutDelta*time.Duration(round)
}
