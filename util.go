package main

import (
	"time"
)

type Ticker struct {
	timer *time.Timer
	C     <-chan HRS
	hrs   HRS
	done  chan struct{}
}

func NewTicker() *Ticker {
	c := make(chan HRS, 1)
	ticker := &Ticker{
		timer: time.NewTimer(0),
		C:     c,
		done:  make(chan struct{}),
	}
	ticker.stopTimer()
	go ticker.run(c)
	return ticker
}

func (ticker *Ticker) Stop() {
	close(ticker.done)
	ticker.stopTimer()
}

func (ticker *Ticker) SetTimeout(hrs HRS, duration time.Duration) {
	if hrs.Cmp(ticker.hrs) <= 0 {
		return
	}
	if ticker.timer != nil {
		ticker.stopTimer()
	} else {
		ticker.timer = time.NewTimer(0)
	}
	ticker.hrs = hrs
	ticker.timer.Reset(duration)
}

func (ticker *Ticker) stopTimer() {
	if !ticker.timer.Stop() {
		select {
		case <-ticker.timer.C:
		default:
			// Timer already stopped
		}
	}
}

func (ticker *Ticker) run(c chan<- HRS) {
	for {
		select {
		case <-ticker.timer.C:
			c <- ticker.hrs
		case <-ticker.done:
			return
		}
	}
}
