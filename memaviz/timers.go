package main

import (
	"fmt"
	"log"
	"time"
)

type Timer struct {
	Name  string
	Start time.Time
}

var last_timers map[string]time.Duration = make(map[string]time.Duration)

func clear_last_timers() {
	last_timers = make(map[string]time.Duration)
}

func (t *Timer) Enter() { t.Start = time.Now() }
func (t *Timer) Exit() {
	if _, ok := last_timers[t.Name]; !ok {
		last_timers[t.Name] = 0
	}
	last_timers[t.Name] += time.Since(t.Start)
}

func PrintTimers(n int) {
	x := ""
	for _, k := range SortedMapKeys(last_timers) {
		x += fmt.Sprintf("%s:%15v ", k, last_timers[k]/time.Duration(n))
	}
	log.Printf("Frame: %s", x)
}
