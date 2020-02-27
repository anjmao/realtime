// +build darwin dragonfly freebsd netbsd openbsd

package realtime

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// TODO: Add retries for syscalls.
// TODO: Make sure resources are released on exit.

type timerHandler func()

type kqueue struct {
	fd         int
	nextID     uint64
	handlers   map[uint64]timerHandler
	handlersMu sync.RWMutex
	logger     func(msg ...interface{})
}

func newKqueue() (*kqueue, error) {
	fd, err := unix.Kqueue()
	if err != nil {
		return nil, fmt.Errorf("could not create queue: %v", err)
	}

	logger := func(msg ...interface{}) {}
	if os.Getenv("KQUEUE_DEBUG") == "1" {
		logger = func(msg ...interface{}) {
			fmt.Println("kqueue:", msg)
		}
	}

	return &kqueue{
		fd:       fd,
		handlers: map[uint64]timerHandler{},
		logger:   logger,
	}, nil
}

func (kq *kqueue) eventsLen() int {
	kq.handlersMu.Lock()
	defer kq.handlersMu.Unlock()
	return len(kq.handlers)
}

func (kq *kqueue) registerTimerEvent(d time.Duration, handler timerHandler) (uint64, error) {
	kq.logger("registerTimerEvent enter")

	kq.handlersMu.Lock()
	kq.nextID++
	id := kq.nextID
	if _, ok := kq.handlers[id]; ok {
		return 0, fmt.Errorf("kqueue: periodic event %d is already registered", id)
	}
	kevent := newOneShotTimerEvent(id, d)
	kq.handlers[id] = handler
	kq.handlersMu.Unlock()

	_, err := unix.Kevent(kq.fd, []unix.Kevent_t{kevent}, []unix.Kevent_t{}, nil)
	if err != nil {
		return 0, fmt.Errorf("could not create one time event %d: %v", id, err)
	}
	kq.logger("registerTimerEvent done", id)
	return id, nil
}

func (kq *kqueue) resetTimerEvent(id uint64, d time.Duration) error {
	kevent := newOneShotTimerEvent(id, d)
	_, err := unix.Kevent(kq.fd, []unix.Kevent_t{kevent}, []unix.Kevent_t{}, nil)
	if err != nil {
		return fmt.Errorf("could not update timer event %d: %v", id, err)
	}
	return nil
}

func (kq *kqueue) registerTickerEvent(d time.Duration, handler timerHandler) (uint64, error) {
	kq.handlersMu.Lock()
	kq.nextID++
	id := kq.nextID
	if _, ok := kq.handlers[id]; ok {
		return 0, fmt.Errorf("kqueue: periodic event %d is already registered", id)
	}
	kevent := newPeriodicTimerEvent(id, d)
	kq.handlers[id] = handler
	kq.handlersMu.Unlock()

	_, err := unix.Kevent(kq.fd, []unix.Kevent_t{kevent}, []unix.Kevent_t{}, nil)
	if err != nil {
		return 0, fmt.Errorf("could not create periodic event %d: %v", id, err)
	}
	return id, nil
}

func (kq *kqueue) deleteEvent(id uint64) error {
	kq.handlersMu.Lock()
	if _, ok := kq.handlers[id]; !ok {
		kq.handlersMu.Unlock()
		return nil
	}
	delete(kq.handlers, id)
	kq.handlersMu.Unlock()

	_, err := unix.Kevent(kq.fd, []unix.Kevent_t{newDeleteEvent(id)}, []unix.Kevent_t{}, nil)
	if err != nil {
		return fmt.Errorf("could not delete event %d: %v", id, err)
	}
	return nil
}

func (kq *kqueue) poll(onError func(error)) {
	const (
		eventsLen    = 1 << 10 // 1024
		maxEventsLen = 1 << 15 // 32768
	)

	defer func() {
		if err := unix.Close(kq.fd); err != nil {
			onError(err)
		}
	}()

	events := make([]unix.Kevent_t, eventsLen)
	for {
		n, err := unix.Kevent(kq.fd, []unix.Kevent_t{}, events, nil)
		if err != nil {
			if temporaryErr(err) {
				continue
			}
			onError(fmt.Errorf("could not get event: %v", err))
			return
		}

		if n == 0 {
			continue
		}

		kq.handlersMu.Lock()
		for i := 0; i < n; i++ {
			e := events[i]
			handler := kq.handlers[e.Ident]
			if handler != nil {
				handler()
			}

			if e.Flags&unix.EV_ONESHOT != 0 {
				delete(kq.handlers, e.Ident)
			}
		}
		kq.handlersMu.Unlock()

		if n == len(events) && n*2 <= maxEventsLen {
			events = make([]unix.Kevent_t, n*2)
		}
	}
}

func newOneShotTimerEvent(id uint64, timer time.Duration) unix.Kevent_t {
	return unix.Kevent_t{
		Ident:  id,
		Filter: unix.EVFILT_TIMER,
		Flags:  unix.EV_ADD | unix.EV_ENABLE | unix.EV_ONESHOT,
		Fflags: unix.NOTE_NSECONDS | unix.NOTE_MACH_CONTINUOUS_TIME,
		Data:   int64(timer),
	}
}

func newPeriodicTimerEvent(id uint64, timer time.Duration) unix.Kevent_t {
	return unix.Kevent_t{
		Ident:  id,
		Filter: unix.EVFILT_TIMER,
		Flags:  unix.EV_ADD | unix.EV_ENABLE,
		Fflags: unix.NOTE_NSECONDS | unix.NOTE_MACH_CONTINUOUS_TIME,
		Data:   int64(timer),
	}
}

func newDeleteEvent(id uint64) unix.Kevent_t {
	return unix.Kevent_t{
		Ident:  id,
		Filter: unix.EVFILT_TIMER,
		Flags:  unix.EV_DELETE,
	}
}

func temporaryErr(err error) bool {
	errno, ok := err.(syscall.Errno)
	if !ok {
		return false
	}
	return errno.Temporary()
}
