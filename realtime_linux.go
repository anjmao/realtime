package realtime

import (
	"time"

	"golang.org/x/sys/unix"
)

var (
	ep *epoll
)

// TODO: Decide how to handle panics. Maybe fallback to std time.

func init() {
	var err error
	ep, err = newEpoll()
	if err != nil {
		panic(err)
	}

	go ep.poll(func(err error) {
		panic(err)
	})
}

func nanotime() uint64 {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &ts); err != nil {
		// TODO: handle panic.
		panic(err)
	}
	return uint64(ts.Sec*1e9 + ts.Nsec)
}

func startTimer(d time.Duration, handler timerHandler) int {
	fd, err := ep.registerTimerEvent(d, handler)
	if err != nil {
		panic(err)
	}
	return fd
}

func stopTimer(id int) {
	if err := ep.deleteEvent(id); err != nil {
		panic(err)
	}
}

func resetTimer(id int, d time.Duration) {
	if err := ep.resetTimerEvent(id, d); err != nil {
		panic(err)
	}
}

func startTicker(d time.Duration, handler timerHandler) int {
	fd, err := ep.registerTickerEvent(d, handler)
	if err != nil {
		panic(err)
	}
	return fd
}

func stopTicker(id int) {
	if err := ep.deleteEvent(id); err != nil {
		panic(err)
	}
}
