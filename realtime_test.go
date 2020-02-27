package realtime

import (
	"fmt"
	"math/rand"
	"sync"
	"syscall"
	"testing"
	"time"
)

func randDuration(max int) time.Duration {
	return time.Duration(rand.Intn(max)) * time.Millisecond
}

func randSleep() {
	time.Sleep(randDuration(1000))
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestNow(t *testing.T) {
	for i := 0; i < 100; i++ {
		t1 := Now()
		t2 := Now()
		if t1.Nano() > t2.Nano() {
			t.Fatalf("t1=%d should have been less than or equal to t2=%d", t1, t2)
		}
	}
}

func TestSince(t *testing.T) {
	for i := 0; i < 100; i++ {
		ts := Now()
		d := Since(ts)
		if d < 0 {
			t.Fatalf("d=%d should be greater than or equal to zero", d)
		}
	}
}

func TestSleep(t *testing.T) {
	const delay = 100 * time.Millisecond
	go func() {
		Sleep(delay / 2)
		interrupt()
	}()
	start := Now()
	Sleep(delay)
	delayadj := delay
	duration := Now().Sub(start)
	if duration < delayadj {
		t.Fatalf("Sleep(%s) slept for only %s", delay, duration)
	}
}

func TestAfterFunc(t *testing.T) {
	i := 10
	c := make(chan bool)
	var f func()
	f = func() {
		i--
		if i >= 0 {
			AfterFunc(0, f)
			Sleep(1 * time.Second)
		} else {
			c <- true
		}
	}

	AfterFunc(0, f)
	<-c
}

func TestAfter(t *testing.T) {
	const delay = 100 * time.Millisecond
	start := Now()
	end := <-After(delay)
	delayadj := delay

	if duration := Now().Sub(start); duration < delayadj {
		t.Fatalf("After(%s) slept for only %d ns", delay, duration)
	}
	if min := start.Add(delayadj); end.Before(min) {
		t.Fatalf("After(%s) expect >= %s, got %s", delay, min, end)
	}
}

func TestConcurrentTimerReset(t *testing.T) {
	// We expect this code to panic rather than crash.
	// Don't worry if it doesn't panic.
	catch := func(i int) {
		if e := recover(); e != nil {
			t.Logf("panic in goroutine %d, as expected, with %q", i, e)
		} else {
			t.Logf("no panic in goroutine %d", i)
		}
	}

	const goroutines = 8
	const tries = 1000
	var wg sync.WaitGroup
	wg.Add(goroutines)
	timer := NewTimer(time.Hour)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			defer catch(i)
			for j := 0; j < tries; j++ {
				timer.Reset(time.Hour + time.Duration(i*j))
			}
		}(i)
	}
	wg.Wait()
}

func TestTicker(t *testing.T) {
	const Count = 10
	Delta := 100 * time.Millisecond
	ticker := NewTicker(Delta)
	t0 := Now()
	for i := 0; i < Count; i++ {
		<-ticker.C
	}
	ticker.Stop()
	t1 := Now()
	dt := t1.Sub(t0)
	target := Delta * Count
	slop := target * 2 / 10
	if dt < target-slop || (!testing.Short() && dt > target+slop) {
		t.Fatalf("%d %s ticks took %s, expected [%s,%s]", Count, Delta, dt, target-slop, target+slop)
	}
	// Now test that the ticker stopped
	time.Sleep(2 * Delta)
	select {
	case <-ticker.C:
		t.Fatal("Ticker did not shut down")
	default:
		// ok
	}
}

func TestTimersTickersNoOverrides(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(4)

	// Create bunch of tickers with random stops.
	go func() {
		t1 := NewTicker(randDuration(1000))
		randSleep()
		t2 := NewTicker(randDuration(1000))
		randSleep()
		t3 := NewTicker(randDuration(1000))

		go func() {
			randSleep()
			t1.Stop()
			randSleep()
			t2.Stop()
			randSleep()
			t3.Stop()

			wg.Done()
		}()

		for {
			select {
			case t := <-t1.C:
				fmt.Println("t1", t1, t)
			case t := <-t2.C:
				fmt.Println("t2", t2, t)
			case t := <-t3.C:
				fmt.Println("t3", t3, t)
			}
		}
	}()

	// More random tickers.
	go func() {
		t4 := NewTicker(randDuration(1000))
		randSleep()
		t5 := NewTicker(randDuration(1000))
		randSleep()
		t6 := NewTicker(randDuration(1000))

		go func() {
			randSleep()
			t4.Stop()
			randSleep()
			t5.Stop()
			randSleep()
			t6.Stop()

			wg.Done()
		}()

		for {
			select {
			case t := <-t4.C:
				fmt.Println("t4", t4, t)
			case t := <-t5.C:
				fmt.Println("t5", t5, t)
			case t := <-t6.C:
				fmt.Println("t6", t6, t)
			}
		}
	}()

	// Create randoms timers.
	go func() {
		timeout := After(300 * time.Millisecond)
		for {
			select {
			case t7 := <-After(randDuration(200)):
				fmt.Println("t7", t7)
			case <-timeout:
				wg.Done()
				return
			}
		}
	}()

	// More random timers
	go func() {
		timeout := After(350 * time.Millisecond)
		for {
			select {
			case t8 := <-After(randDuration(300)):
				fmt.Println("t7", t8)
			case <-timeout:
				wg.Done()
				return
			}
		}
	}()

	wg.Wait()
}

func benchmark(b *testing.B, bench func(n int)) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bench(1000)
		}
	})
	b.StopTimer()
}

func BenchmarkAfter(b *testing.B) {
	benchmark(b, func(n int) {
		for i := 0; i < n; i++ {
			<-After(1)
		}
	})
}

func BenchmarkTimerStop(b *testing.B) {
	benchmark(b, func(n int) {
		for i := 0; i < n; i++ {
			NewTimer(1 * time.Second).Stop()
		}
	})
}

func BenchmarkTickerStop(b *testing.B) {
	benchmark(b, func(n int) {
		for i := 0; i < n; i++ {
			NewTicker(1 * time.Second).Stop()
		}
	})
}

func BenchmarkSimultaneousAfterFunc(b *testing.B) {
	benchmark(b, func(n int) {
		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			time.AfterFunc(0, wg.Done)
		}
		wg.Wait()
	})
}

func BenchmarkStartStop(b *testing.B) {
	benchmark(b, func(n int) {
		timers := make([]*Timer, n)
		for i := 0; i < n; i++ {
			timers[i] = AfterFunc(time.Hour, nil)
		}

		for i := 0; i < n; i++ {
			timers[i].Stop()
		}
	})
}

func BenchmarkTimerReset(b *testing.B) {
	benchmark(b, func(n int) {
		t := NewTimer(time.Hour)
		for i := 0; i < n; i++ {
			t.Reset(time.Hour)
		}
		t.Stop()
	})
}

func BenchmarkSleep(b *testing.B) {
	benchmark(b, func(n int) {
		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func() {
				Sleep(time.Nanosecond)
				wg.Done()
			}()
		}
		wg.Wait()
	})
}

func BenchmarkNow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Now()
	}
}

func BenchmarkStdNow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = time.Now()
	}
}

func BenchmarkSince(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ts := Now()
		Since(ts)
	}
}

func BenchmarkStdSince(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ts := time.Now()
		time.Since(ts)
	}
}

func interrupt() {
	syscall.Kill(syscall.Getpid(), syscall.SIGCHLD)
}
