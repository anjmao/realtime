// +build darwin dragonfly freebsd netbsd openbsd

package realtime

import (
	"sync"
	"testing"
	"time"
)

func TestTimerEventCleanupAfterFire(t *testing.T) {
	queue, err := newKqueue()
	if err != nil {
		t.Fatal(err)
	}

	go queue.poll(func(err error) {

	})

	var wg sync.WaitGroup
	wg.Add(3)
	queue.registerTimerEvent(10*time.Millisecond, func() {
		wg.Done()
	})
	queue.registerTimerEvent(15*time.Millisecond, func() {
		wg.Done()
	})
	queue.registerTimerEvent(20*time.Millisecond, func() {
		wg.Done()
	})

	expectedLen := 3
	actualLen := queue.eventsLen()
	if expectedLen != actualLen {
		t.Fatalf("expected %d events, got %d", expectedLen, actualLen)
	}

	wg.Wait()
	expectedLen = 0
	actualLen = queue.eventsLen()
	if expectedLen != actualLen {
		t.Fatalf("expected %d events, got %d", expectedLen, actualLen)
	}
}

func TestTimerEventDeleteNotFiresEvent(t *testing.T) {
	queue, err := newKqueue()
	if err != nil {
		t.Fatal(err)
	}

	go queue.poll(func(err error) {

	})

	id, err := queue.registerTimerEvent(100*time.Millisecond, func() {
		t.Fatal("should not call callback")
	})

	queue.deleteEvent(id)
	time.Sleep(150 * time.Millisecond)
}

func TestTickerEvent(t *testing.T) {
	queue, err := newKqueue()
	if err != nil {
		t.Fatal(err)
	}

	go queue.poll(func(err error) {

	})

	var ticks int
	id, err := queue.registerTickerEvent(100*time.Millisecond, func() {
		ticks++
	})

	expectedLen := 1
	actualLen := queue.eventsLen()
	if expectedLen != actualLen {
		t.Fatalf("expected %d events, got %d", expectedLen, actualLen)
	}

	// Delete event after 2 ticks.
	time.Sleep(250 * time.Millisecond)
	queue.deleteEvent(id)

	// Wait some more time to ensure there is no more ticks.
	time.Sleep(400 * time.Millisecond)
	expectedTicks := 2
	actualTicks := ticks
	if expectedTicks != actualTicks {
		t.Fatalf("expected %d ticks, got %d", expectedTicks, actualTicks)
	}
}
