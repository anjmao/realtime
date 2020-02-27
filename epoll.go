// +build linux

package realtime

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// TODO: Add retries for syscalls.
// TODO: Make sure resources are released on exit.

type timerHandler func()

type epoll struct {
	fd int
	// eventFd int

	handlers   map[int]timerHandler
	handlersMu sync.RWMutex
	logger     func(msg ...interface{})
}

func newEpoll() (*epoll, error) {
	fd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, fmt.Errorf("could not create epoll: %v", err)
	}

	logger := func(msg ...interface{}) {}
	if os.Getenv("EPOLL_DEBUG") == "1" {
		logger = func(msg ...interface{}) {
			fmt.Println("epoll:", msg)
		}
	}

	ep := &epoll{
		fd:       fd,
		handlers: map[int]timerHandler{},
		logger:   logger,
	}

	return ep, nil
}

func (ep *epoll) registerTimerEvent(d time.Duration, handler timerHandler) (int, error) {
	ep.logger("registerTimerEvent enter")
	tfd, err := ep.createTimer(d)

	ep.handlersMu.Lock()
	if _, ok := ep.handlers[tfd]; ok {
		return 0, fmt.Errorf("timer event %d is already registered", tfd)
	}
	ep.handlers[tfd] = handler
	ep.handlersMu.Unlock()

	ev := &unix.EpollEvent{
		Events: unix.EPOLLIN | unix.EPOLLONESHOT,
		Fd:     int32(tfd),
	}
	err = unix.EpollCtl(ep.fd, unix.EPOLL_CTL_ADD, tfd, ev)
	if err != nil {
		return 0, fmt.Errorf("could not create one time event %d: %v", tfd, err)
	}
	ep.logger("registerTimerEvent done", tfd)
	return tfd, nil
}

func (ep *epoll) registerTickerEvent(d time.Duration, handler timerHandler) (int, error) {
	ep.logger("registerTickerEvent enter")
	tfd, err := ep.createTimer(d)

	ep.handlersMu.Lock()
	if _, ok := ep.handlers[tfd]; ok {
		return 0, fmt.Errorf("periodic event %d is already registered", tfd)
	}
	ep.handlers[tfd] = handler
	ep.handlersMu.Unlock()

	ev := &unix.EpollEvent{
		Events: unix.EPOLLIN,
		Fd:     int32(tfd),
	}
	err = unix.EpollCtl(ep.fd, unix.EPOLL_CTL_ADD, tfd, ev)
	if err != nil {
		return 0, fmt.Errorf("could not create periodic event %d: %v", tfd, err)
	}
	ep.logger("registerTickerEvent done", tfd)
	return tfd, nil
}

func (ep *epoll) deleteEvent(id int) error {
	ep.handlersMu.Lock()
	if _, ok := ep.handlers[id]; !ok {
		ep.handlersMu.Unlock()
		return nil
	}
	delete(ep.handlers, id)
	ep.handlersMu.Unlock()

	if err := unix.EpollCtl(ep.fd, unix.EPOLL_CTL_DEL, id, nil); err != nil {
		return fmt.Errorf("could not delete event %d: %v", id, err)
	}
	return nil
}

func (ep *epoll) resetTimerEvent(id int, d time.Duration) error {
	// TODO: Reset flow could be:
	// 1. Close old timer and create new.
	// 2. Call EpollCtl EPOLL_CTL_MOD with new event and timer fd.

	return errors.New("not implemented")
}

func (ep *epoll) createTimer(d time.Duration) (int, error) {
	tfd, err := timerFdCreate(unix.CLOCK_BOOTTIME, unix.O_NONBLOCK)
	if err != nil {
		ep.logger("fallback to CLOCK_MONOTONIC")
		tfd, err = timerFdCreate(unix.CLOCK_MONOTONIC, unix.O_NONBLOCK)
		if err != nil {
			return 0, fmt.Errorf("could not create timer file descriptor: %v", err)
		}
	}

	val := unix.Timespec{Sec: int64(0), Nsec: d.Nanoseconds()}
	if val.Nsec >= 1e9 {
		val.Sec = val.Nsec / 1e9
		val.Nsec = val.Nsec % 1e9
	}
	spec := timerSpec{
		ItInterval: val,
		ItValue:    val,
	}

	if err := timerFdSetTime(tfd, 0, &spec, &timerSpec{}); err != nil {
		unix.Close(tfd)
		return 0, fmt.Errorf("could not set timer: %v", err)
	}

	return tfd, nil
}

func (ep *epoll) poll(onError func(error)) {
	const (
		eventsLen    = 1 << 10 // 1024
		maxEventsLen = 1 << 15 // 32768
	)

	defer func() {
		if err := unix.Close(ep.fd); err != nil {
			onError(err)
		}
	}()

	events := make([]unix.EpollEvent, eventsLen)
	tickerBuf := make([]byte, 256)
	for {
		n, err := unix.EpollWait(ep.fd, events, -1)
		if err != nil {
			if temporaryErr(err) {
				continue
			}
			onError(err)
			return
		}

		if n == 0 {
			continue
		}

		// TODO: Lock only for call handlers. Could register pending handlers into separate
		// TODO: slice and call outside lock.
		ep.handlersMu.RLock()
		for i := 0; i < n; i++ {
			e := events[i]
			handler := ep.handlers[int(e.Fd)]
			if handler != nil {
				handler()
			}

			if e.Events&unix.EPOLLONESHOT != 0 {
				// Remove handler for one shot timer.
				delete(ep.handlers, int(e.Fd))
			} else {
				// Read periodic ticker.
				if _, err := unix.Read(int(e.Fd), tickerBuf); err != nil {
					// TODO: Will probably need to use tickerBuf and return read result to handler.
					// TODO: Handle error?
				}
			}
		}
		ep.handlersMu.RUnlock()

		if n == len(events) && n*2 <= maxEventsLen {
			events = make([]unix.EpollEvent, n*2)
		}
	}
}

func timerFdCreate(clockId int, flags int) (int, error) {
	tmFd, _, err := unix.Syscall(unix.SYS_TIMERFD_CREATE, uintptr(clockId), uintptr(flags), 0)
	if err != 0 {
		return -1, err
	}
	return int(tmFd), nil
}

type timerSpec struct {
	ItInterval unix.Timespec
	ItValue    unix.Timespec
}

func timerFdSetTime(fd int, flags int, new *timerSpec, old *timerSpec) error {
	_, _, err := unix.Syscall6(unix.SYS_TIMERFD_SETTIME,
		uintptr(fd), uintptr(flags), uintptr(unsafe.Pointer(new)), uintptr(unsafe.Pointer(old)), 0, 0)
	if err != 0 {
		return err
	}
	return nil
}

func temporaryErr(err error) bool {
	errno, ok := err.(syscall.Errno)
	if !ok {
		return false
	}
	return errno.Temporary()
}
