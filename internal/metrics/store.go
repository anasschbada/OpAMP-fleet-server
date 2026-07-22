package metrics

import "sync"

// Store holds the latest metrics Snapshot per agent, purely in memory.
// Unlike the fleet registry (internal/store), losing this on restart is
// fine: it's a live operational view, not audit history, and repopulates
// within one scrape interval.
type Store struct {
	mu   sync.RWMutex
	byID map[string]Snapshot
}

func NewStore() *Store {
	return &Store{byID: make(map[string]Snapshot)}
}

func (s *Store) Get(instanceUID string) (Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.byID[instanceUID]
	return snap, ok
}

func (s *Store) update(instanceUID string, mutate func(*Snapshot)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.byID[instanceUID]
	mutate(&snap)
	s.byID[instanceUID] = snap
}

// Prune removes any agent not present in liveInstanceUIDs, so metrics for
// an agent that disconnected long ago (and was cleaned up from the fleet
// registry) don't accumulate forever in memory.
func (s *Store) Prune(liveInstanceUIDs map[string]struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for uid := range s.byID {
		if _, ok := liveInstanceUIDs[uid]; !ok {
			delete(s.byID, uid)
		}
	}
}
