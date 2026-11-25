package main

import (
	"context"
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"runtime"
	"slices"
	"sync"
	"time"
)

const delay = 100 * time.Millisecond

type Tick struct {
	Symbol string
	Price  float64
	At     time.Time
}

type Alert struct {
	Symbol string
	Price  float64
	Avg    float64
	Pct    float64
}

func (a Alert) String() string {
	return fmt.Sprintf("ALERT %-5s %9.2f  avg %9.2f  %+.1f%%", a.Symbol, a.Price, a.Avg, a.Pct*100)
}

func generateRandomFloat(low float64, high float64) float64 {
	return low + rand.Float64()*(high-low)
}

func ticker(ctx context.Context, symbol string, start float64) <-chan Tick {
	out := make(chan Tick)
	go func() {
		defer close(out)
		for {
			select {
			case <-time.After(delay):
				moveBy := generateRandomFloat(-0.02, 0.02)
				start *= 1 + moveBy
				t := Tick{Symbol: symbol, Price: start, At: time.Now()}
				select {
				case out <- t:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out

}

func route(ctx context.Context, in <-chan Tick, symbols []string) map[string]<-chan Tick {
	outs := make(map[string]chan Tick, len(symbols))     // router sends on these
	result := make(map[string]<-chan Tick, len(symbols)) // caller receives on these
	for _, sym := range symbols {
		c := make(chan Tick, 16)
		outs[sym] = c
		result[sym] = c
	}
	go func() {
		defer func() {
			for _, c := range outs {
				close(c)
			}
		}()

		for t := range in {
			c, ok := outs[t.Symbol]
			if !ok {
				continue // unknown symbol: no channel for it
			}
			select {
			case c <- t:
			case <-ctx.Done():
				return
			}
		}
	}()

	return result
}

func merge(ctx context.Context, ins ...<-chan Tick) <-chan Tick {
	m := make(chan Tick)
	var wg sync.WaitGroup
	for _, i := range ins {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range i {
				select {
				case m <- t:
				case <-ctx.Done():
					return
				}
			}
		}()

	}
	go func() {
		wg.Wait()
		close(m)
	}()
	return m
}

func average(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// worker keeps a moving average of the last 10 prices for one symbol and
// sends an Alert when a tick is more than 3% away from it. Sends never block:
// if the alerts buffer is full, the alert is dropped. Returns the drop count.
func worker(in <-chan Tick, alerts chan<- Alert) (dropped int) {
	window := make([]float64, 0, 10)
	for t := range in {
		if len(window) == 10 {
			avg := average(window)
			pct := (t.Price - avg) / avg
			if math.Abs(pct) > 0.03 {
				a := Alert{Symbol: t.Symbol, Price: t.Price, Avg: avg, Pct: pct}
				select {
				case alerts <- a:
				default:
					dropped++ // notifier is behind: drop rather than slow down
				}
			}
			window = window[1:]
		}
		window = append(window, t.Price)
	}
	return dropped
}

func notifier(alerts <-chan Alert) {
	flushEvery := time.NewTicker(500 * time.Millisecond)
	defer flushEvery.Stop()

	pending := make(map[string]Alert)
	flush := func() {
		for _, sym := range slices.Sorted(maps.Keys(pending)) {
			fmt.Println(pending[sym])
		}
		clear(pending)
	}

	for {
		select {
		case a, ok := <-alerts:
			if !ok {
				flush()
				return
			}
			pending[a.Symbol] = a
		case <-flushEvery.C:
			if len(pending) > 0 {
				fmt.Println("---", time.Now().Format("15:04:05.000"))
				flush()
			}
		}
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	symbols := []string{"ACME", "BOLT", "CRUX", "DYNA", "EPIC"}
	all := merge(ctx,
		ticker(ctx, "ACME", 100),
		ticker(ctx, "BOLT", 42),
		ticker(ctx, "CRUX", 250),
		ticker(ctx, "DYNA", 8),
		ticker(ctx, "EPIC", 1200),
	)
	outs := route(ctx, all, symbols)

	alerts := make(chan Alert, 100)
	dropped := make([]int, len(symbols)) // one slot per worker: no shared writes
	var wg sync.WaitGroup
	for i, sym := range symbols {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dropped[i] = worker(outs[sym], alerts)
		}()
	}
	go func() {
		wg.Wait()
		close(alerts)
	}()

	notifier(alerts) // returns once all workers are done and alerts is closed

	total := 0
	for _, d := range dropped {
		total += d
	}
	fmt.Println("dropped alerts:", total)

	time.Sleep(10 * time.Millisecond)
	fmt.Println("done, goroutines:", runtime.NumGoroutine())
}
