package store

import "sync"

// MemoryStore is a process-local Store. It loses all history on restart, so
// it is meant for development/testing. Production deployments should use the
// SQLite-backed store (sqlite.go) with a PersistentVolumeClaim so fleet
// history survives pod restarts.
type MemoryStore struct {
	mu     sync.RWMutex
	agents map[string]Agent
	pushes map[string][]ConfigPush // keyed by agent instance uid, oldest first
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		agents: make(map[string]Agent),
		pushes: make(map[string][]ConfigPush),
	}
}

func (s *MemoryStore) UpsertAgent(instanceUID string, mutate func(a *Agent)) (Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a := s.agents[instanceUID]
	a.InstanceUID = instanceUID
	mutate(&a)
	s.agents[instanceUID] = a
	return a, nil
}

func (s *MemoryStore) GetAgent(instanceUID string) (Agent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.agents[instanceUID]
	return a, ok
}

func (s *MemoryStore) ListAgents() ([]Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Agent, 0, len(s.agents))
	for _, a := range s.agents {
		out = append(out, a)
	}
	return out, nil
}

func (s *MemoryStore) DeleteAgent(instanceUID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.agents, instanceUID)
	delete(s.pushes, instanceUID)
	return nil
}

func (s *MemoryStore) AppendConfigPush(push ConfigPush) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pushes[push.AgentUID] = append(s.pushes[push.AgentUID], push)
	return nil
}

func (s *MemoryStore) UpdateConfigPushResult(agentUID, pushID string, succeeded bool, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.pushes[agentUID]
	for i := range list {
		if list[i].ID == pushID {
			list[i].Succeeded = succeeded
			list[i].ErrorMessage = errMsg
			return nil
		}
	}
	return nil
}

func (s *MemoryStore) ListConfigPushes(agentUID string) ([]ConfigPush, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := s.pushes[agentUID]
	out := make([]ConfigPush, len(list))
	copy(out, list)
	return out, nil
}

func (s *MemoryStore) Close() error { return nil }
