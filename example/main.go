package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/anjmao/realtime"
)

func randSleep() {
	time.Sleep(time.Duration(rand.Intn(10)) * time.Second)
}

func randDuration(max int) time.Duration {
	return time.Duration(rand.Intn(max)) * time.Millisecond
}

func main() {
	rand.Seed(time.Now().UnixNano())

	go func() {
		http.ListenAndServe(":6060", nil)
	}()

	go func() {
		for {
			<-time.After(2 * time.Second)
			fmt.Println("timer", time.Now())
		}
	}()

	go func() {
		for range realtime.Tick(1 * time.Second) {
			fmt.Println("ticker", time.Now())
		}
	}()

	var end string
	fmt.Fscan(os.Stdin, &end)
}
