package store

import (
	"path/filepath"
	"testing"
)

// testStores runs every test against both implementations of Store, so a
// behavioral difference between them (a bug in one, or a spec drift)
// surfaces immediately.
func testStores(t *testing.T) map[string]Store {
	t.Helper()
	sqliteStore, err := NewSQLiteStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { sqliteStore.Close() })
	return map[string]Store{
		"memory": NewMemoryStore(),
		"sqlite": sqliteStore,
	}
}

func TestStore_UpsertAndGetAgent(t *testing.T) {
	for name, s := range testStores(t) {
		t.Run(name, func(t *testing.T) {
			agent, err := s.UpsertAgent("uid-1", func(a *Agent) {
				a.ServiceName = "collector-1"
				a.Namespace = "ns-a"
				a.Attributes = map[string]string{"k": "v"}
			})
			if err != nil {
				t.Fatalf("UpsertAgent: %v", err)
			}
			if agent.InstanceUID != "uid-1" || agent.ServiceName != "collector-1" {
				t.Errorf("unexpected agent returned: %+v", agent)
			}

			got, ok := s.GetAgent("uid-1")
			if !ok {
				t.Fatal("expected agent to be found")
			}
			if got.Namespace != "ns-a" || got.Attributes["k"] != "v" {
				t.Errorf("round-tripped agent mismatch: %+v", got)
			}

			if _, ok := s.GetAgent("does-not-exist"); ok {
				t.Error("expected GetAgent to report not-found for an unknown uid")
			}
		})
	}
}

func TestStore_UpsertAgent_PartialUpdatePreservesOtherFields(t *testing.T) {
	for name, s := range testStores(t) {
		t.Run(name, func(t *testing.T) {
			_, err := s.UpsertAgent("uid-1", func(a *Agent) {
				a.ServiceName = "collector-1"
				a.Namespace = "ns-a"
			})
			if err != nil {
				t.Fatalf("first UpsertAgent: %v", err)
			}

			// Simulates an OpAMP message that only carries Health, not
			// AgentDescription -- ServiceName/Namespace must survive.
			_, err = s.UpsertAgent("uid-1", func(a *Agent) {
				a.Healthy = true
			})
			if err != nil {
				t.Fatalf("second UpsertAgent: %v", err)
			}

			got, _ := s.GetAgent("uid-1")
			if got.ServiceName != "collector-1" || got.Namespace != "ns-a" {
				t.Errorf("partial update lost earlier fields: %+v", got)
			}
			if !got.Healthy {
				t.Error("partial update did not apply Healthy")
			}
		})
	}
}

func TestStore_ListAgents(t *testing.T) {
	for name, s := range testStores(t) {
		t.Run(name, func(t *testing.T) {
			for _, uid := range []string{"a", "b", "c"} {
				if _, err := s.UpsertAgent(uid, func(agent *Agent) {}); err != nil {
					t.Fatalf("UpsertAgent(%s): %v", uid, err)
				}
			}
			agents, err := s.ListAgents()
			if err != nil {
				t.Fatalf("ListAgents: %v", err)
			}
			if len(agents) != 3 {
				t.Errorf("expected 3 agents, got %d", len(agents))
			}
		})
	}
}

func TestStore_DeleteAgent(t *testing.T) {
	for name, s := range testStores(t) {
		t.Run(name, func(t *testing.T) {
			if _, err := s.UpsertAgent("uid-1", func(a *Agent) {}); err != nil {
				t.Fatalf("UpsertAgent: %v", err)
			}
			if err := s.AppendConfigPush(ConfigPush{ID: "p1", AgentUID: "uid-1"}); err != nil {
				t.Fatalf("AppendConfigPush: %v", err)
			}

			if err := s.DeleteAgent("uid-1"); err != nil {
				t.Fatalf("DeleteAgent: %v", err)
			}
			if _, ok := s.GetAgent("uid-1"); ok {
				t.Error("agent should be gone after DeleteAgent")
			}
			pushes, err := s.ListConfigPushes("uid-1")
			if err != nil {
				t.Fatalf("ListConfigPushes: %v", err)
			}
			if len(pushes) != 0 {
				t.Error("config pushes should be gone after DeleteAgent too")
			}
		})
	}
}

func TestStore_ConfigPushLifecycle(t *testing.T) {
	for name, s := range testStores(t) {
		t.Run(name, func(t *testing.T) {
			if _, err := s.UpsertAgent("uid-1", func(a *Agent) {}); err != nil {
				t.Fatalf("UpsertAgent: %v", err)
			}

			push := ConfigPush{ID: "push-1", AgentUID: "uid-1", ConfigYAML: "receivers:\n  otlp:\n", PushedBy: "operator"}
			if err := s.AppendConfigPush(push); err != nil {
				t.Fatalf("AppendConfigPush: %v", err)
			}

			pushes, err := s.ListConfigPushes("uid-1")
			if err != nil {
				t.Fatalf("ListConfigPushes: %v", err)
			}
			if len(pushes) != 1 || pushes[0].Succeeded {
				t.Fatalf("expected one pending push, got %+v", pushes)
			}

			if err := s.UpdateConfigPushResult("uid-1", "push-1", true, ""); err != nil {
				t.Fatalf("UpdateConfigPushResult: %v", err)
			}
			pushes, _ = s.ListConfigPushes("uid-1")
			if !pushes[0].Succeeded {
				t.Error("expected push to be marked succeeded")
			}

			// A different agent must never see another agent's history.
			otherPushes, err := s.ListConfigPushes("uid-2")
			if err != nil {
				t.Fatalf("ListConfigPushes(uid-2): %v", err)
			}
			if len(otherPushes) != 0 {
				t.Errorf("expected no pushes for an unrelated agent, got %d", len(otherPushes))
			}
		})
	}
}
