package realtime

import (
	"time"

	"golang.org/x/sys/unix"
)

var (
	kq *kqueue
)

// TODO: Decide how to handle panics. Maybe fallback to std time.

func init() {
	var err error
	kq, err = newKqueue()
	if err != nil {
		panic(err)
	}

	go kq.poll(func(err error) {
		panic(err)
	})
}

func nanotime() uint64 {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC_RAW, &ts); err != nil {
		// TODO: handle panic.
		panic(err)
	}
	return uint64(ts.Sec)*1e9 + (ts.Nsec)
}

func startTimer(d time.Duration, handler timerHandler) int {
	fd, err := kq.registerTimerEvent(d, handler)
	if err != nil {
		panic(err)
	}
	return int(fd)
}

func stopTimer(id int) {
	if err := kq.deleteEvent(uint64(id)); err != nil {
		panic(err)
	}
}

func resetTimer(id int, d time.Duration) {
	if err := kq.resetTimerEvent(uint64(id), d); err != nil {
		panic(err)
	}
}

func startTicker(d time.Duration, handler timerHandler) int {
	fd, err := kq.registerTickerEvent(d, handler)
	if err != nil {
		panic(err)
	}
	return int(fd)
}

func stopTicker(id int) {
	if err := kq.deleteEvent(uint64(id)); err != nil {
		panic(err)
	}
}
