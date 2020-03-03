package realtime

import (
	"fmt"
	"time"
)

type Time struct {
	ns time.Duration
}

func Now() Time {
	return Time{ns: time.Duration(nanotime())}
}

func (t Time) Sub(u Time) time.Duration {
	return t.ns - u.ns
}

func (t Time) Nano() time.Duration {
	return t.ns
}

func (t Time) Before(u Time) bool {
	return t.ns < u.ns
}

func (t Time) After(u Time) bool {
	return t.ns > u.ns
}

func (t Time) Add(d time.Duration) Time {
	t.ns = t.ns + d
	return t
}

func (t Time) String() string {
	return t.ns.String()
}

func Since(u Time) time.Duration {
	return Now().ns - u.ns
}

func Sleep(d time.Duration) {
	<-NewTimer(d).C
}

func AfterFunc(d time.Duration, f func()) *Timer {
	t := &Timer{}
	t.id = startTimer(d, func() {
		if f != nil {
			go f()
		}
	})
	return t
}

func After(d time.Duration) <-chan Time {
	return NewTimer(d).C
}

func NewTimer(d time.Duration) *Timer {
	c := make(chan Time, 1)
	t := &Timer{
		C: c,
	}
	t.startTimer(d)
	return t
}

type Timer struct {
	C       chan Time
	stopped bool
	id      int
}

func (t *Timer) String() string {
	return fmt.Sprintf("timer#%d", t.id)
}

func (t *Timer) Stop() bool {
	if t.stopped {
		return true
	}
	stopTimer(t.id)
	return !t.stopped
}

func (t *Timer) Reset(d time.Duration) bool {
	resetTimer(t.id, d)
	return true
}

func (t *Timer) startTimer(d time.Duration) {
	t.stopped = false
	t.id = startTimer(d, func() {
		select {
		case t.C <- Now():
		default:
		}
	})
}

func Tick(d time.Duration) <-chan Time {
	return NewTicker(d).C
}

func NewTicker(d time.Duration) *Ticker {
	c := make(chan Time, 1)
	t := &Ticker{
		C: c,
	}
	t.id = startTicker(d, func() {
		select {
		case t.C <- Now():
		default:
		}
	})
	return t
}

type Ticker struct {
	C  chan Time
	id int
}

func (t *Ticker) Stop() {
	stopTicker(t.id)
}

func (t *Ticker) String() string {
	return fmt.Sprintf("ticker#%d", t.id)
}
