package gower

import (
	"strconv"
	"sync"
	"time"
)

// A structure to keep various stats about the server
type Stat struct {
	LastMinReqs  int
	LastHourReqs int
	LastDayReqs  int
	LastWeekReqs int
	TotalReqs    int

	Reqs            map[string]int
	ServerStartedAt time.Time
	sync.Mutex
}

// Create a new stat datastructure
func NewStat() *Stat {
	return &Stat{
		LastMinReqs:  0,
		LastHourReqs: 0,
		LastDayReqs:  0,
		LastWeekReqs: 0,
		TotalReqs:    0,

		Reqs:            map[string]int{},
		ServerStartedAt: time.Now(),
	}
}

// Get server uptime
func (s *Stat) GetUptime() time.Duration {
	return time.Since(s.ServerStartedAt)
}

// Increment the counter for requests served
func (s *Stat) Increment(statusCode int, duration time.Duration) {
	s.Lock()
	defer s.Unlock()

	s.TotalReqs += 1
	status := strconv.Itoa(statusCode)
	s.Reqs[status] += 1

	// (*) Note
	// Investigate further !!!
	// It appears that TotalReqs & Reqs[X] is a bit higher
	// than the number of actual requests when tested for instance with
	// "ab -c 100 -n 100 http://127.0.0.1/"
	// Could it be a mutex problem?
	// Are multiple goroutines updating the Stat object at the same time?
}
