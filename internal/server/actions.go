package server

import (
	"log"
	"time"

	"github.com/HarshalPatel1972/rift/internal/protocol"
)

const maxSleepTimer = 12 * 60 // minutes

// action runs a bedtime action. Failures are logged, not surfaced as the
// "input blocked" warning, which is about keyboard/mouse injection.
func (s *Server) action(a byte, arg uint32) error {
	pw := s.host.Power
	if pw == nil {
		return nil
	}
	var err error
	switch a {
	case protocol.ActionLock:
		err = pw.Lock()
	case protocol.ActionDisplayOff:
		err = pw.DisplayOff()
	case protocol.ActionSleep:
		s.SetSleepTimer(0)
		err = pw.Sleep()
	case protocol.ActionSleepTimer:
		s.SetSleepTimer(time.Duration(min(arg, maxSleepTimer)) * time.Minute)
	default:
		return nil
	}
	if err != nil {
		log.Printf("action %d: %v", a, err)
	}
	return nil
}

// SetSleepTimer puts the PC to sleep after d; d <= 0 cancels. The phone and
// dashboard both show the countdown.
func (s *Server) SetSleepTimer(d time.Duration) {
	s.mu.Lock()
	if s.sleepTimer != nil {
		s.sleepTimer.Stop()
		s.sleepTimer = nil
	}
	s.status.SleepAt = time.Time{}
	if d > 0 && s.host.Power != nil {
		at := time.Now().Add(d)
		s.status.SleepAt = at
		var t *time.Timer
		t = time.AfterFunc(d, func() {
			s.mu.Lock()
			if s.sleepTimer != t {
				s.mu.Unlock()
				return // cancelled or replaced
			}
			s.sleepTimer = nil
			s.status.SleepAt = time.Time{}
			s.notifyLocked()
			s.mu.Unlock()
			log.Print("sleep timer: going to sleep")
			if err := s.host.Power.Sleep(); err != nil {
				log.Printf("sleep timer: %v", err)
			}
		})
		s.sleepTimer = t
		log.Printf("sleep timer: %s", d)
	}
	s.notifyLocked()
	s.mu.Unlock()
	s.pingActive()
}
