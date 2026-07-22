package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver: no cgo, so the server stays a static, distroless-friendly binary
)

// schema is applied on every startup. CREATE TABLE/INDEX IF NOT EXISTS makes
// this safe to re-run against an existing database file.
const schema = `
CREATE TABLE IF NOT EXISTS agents (
	instance_uid            TEXT PRIMARY KEY,
	service_name            TEXT NOT NULL DEFAULT '',
	namespace               TEXT NOT NULL DEFAULT '',
	version                 TEXT NOT NULL DEFAULT '',
	node_name               TEXT NOT NULL DEFAULT '',
	pod_name                TEXT NOT NULL DEFAULT '',
	attributes_json         TEXT NOT NULL DEFAULT '{}',
	capabilities            INTEGER NOT NULL DEFAULT 0,
	connection_state        TEXT NOT NULL DEFAULT 'disconnected',
	last_seen_unix          INTEGER NOT NULL DEFAULT 0,
	start_time_unix         INTEGER NOT NULL DEFAULT 0,
	healthy                 INTEGER NOT NULL DEFAULT 0,
	last_error              TEXT NOT NULL DEFAULT '',
	effective_config_yaml   TEXT NOT NULL DEFAULT '',
	config_sync             TEXT NOT NULL DEFAULT 'pending',
	last_remote_config_hash BLOB,
	pushed_by               TEXT NOT NULL DEFAULT '',
	last_pushed_at_unix     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS config_pushes (
	id             TEXT PRIMARY KEY,
	agent_uid      TEXT NOT NULL,
	timestamp_unix INTEGER NOT NULL,
	config_yaml    TEXT NOT NULL,
	pushed_by      TEXT NOT NULL DEFAULT '',
	note           TEXT NOT NULL DEFAULT '',
	succeeded      INTEGER NOT NULL DEFAULT 0,
	error_message  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_config_pushes_agent ON config_pushes(agent_uid, timestamp_unix);
`

// SQLiteStore persists the fleet registry to a single file, suitable for a
// PersistentVolumeClaim mounted into the server's pod. It uses the pure-Go
// modernc.org/sqlite driver (no cgo) so the binary stays statically linked,
// which keeps the container image minimal (distroless, nothing to patch in
// a libc-dependent shared object).
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (creating if absent) the database file at path and
// applies the schema. WAL mode lets REST reads proceed while an OpAMP
// callback is writing, and busy_timeout avoids spurious "database is
// locked" errors under the server's normal concurrency.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	// SQLite has no server process to pool connections against; a single
	// writer avoids "database is locked" retries under load.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close() // best-effort: we're already returning the schema error, a close failure adds nothing actionable
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) UpsertAgent(instanceUID string, mutate func(a *Agent)) (Agent, error) {
	var result Agent
	err := s.withTx(func(tx *sql.Tx) error {
		a, err := getAgentTx(tx, instanceUID)
		if err != nil {
			return err
		}
		a.InstanceUID = instanceUID
		mutate(&a)
		result = a
		return upsertAgentTx(tx, a)
	})
	if err != nil {
		return Agent{}, fmt.Errorf("upsert agent %s: %w", instanceUID, err)
	}
	return result, nil
}

func (s *SQLiteStore) withTx(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback() // best-effort: the transaction is abandoned either way, fn's error is what the caller needs
		return err
	}
	return tx.Commit()
}

func getAgentTx(tx *sql.Tx, instanceUID string) (Agent, error) {
	row := tx.QueryRow(`SELECT instance_uid, service_name, namespace, version, node_name, pod_name,
		attributes_json, capabilities, connection_state, last_seen_unix, start_time_unix, healthy,
		last_error, effective_config_yaml, config_sync, last_remote_config_hash, pushed_by, last_pushed_at_unix
		FROM agents WHERE instance_uid = ?`, instanceUID)
	a, err := scanAgent(row)
	if err == sql.ErrNoRows {
		return Agent{InstanceUID: instanceUID, Attributes: map[string]string{}}, nil
	}
	return a, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAgent(row rowScanner) (Agent, error) {
	var a Agent
	var attrsJSON string
	var lastSeen, startTime, lastPushed int64
	var healthy int
	err := row.Scan(&a.InstanceUID, &a.ServiceName, &a.Namespace, &a.Version, &a.NodeName, &a.PodName,
		&attrsJSON, &a.Capabilities, &a.ConnectionState, &lastSeen, &startTime, &healthy,
		&a.LastError, &a.EffectiveConfigYAML, &a.ConfigSync, &a.LastRemoteConfigHash, &a.PushedBy, &lastPushed)
	if err != nil {
		return Agent{}, err
	}
	a.Healthy = healthy != 0
	a.LastSeen = unixOrZero(lastSeen)
	a.StartTime = unixOrZero(startTime)
	a.LastPushedAt = unixOrZero(lastPushed)
	a.Attributes = map[string]string{}
	if attrsJSON != "" {
		_ = json.Unmarshal([]byte(attrsJSON), &a.Attributes) // best-effort: corrupt JSON just yields empty attributes, never a crash
	}
	return a, nil
}

func unixOrZero(sec int64) time.Time {
	if sec == 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}

func upsertAgentTx(tx *sql.Tx, a Agent) error {
	attrsJSON, err := json.Marshal(a.Attributes)
	if err != nil {
		return fmt.Errorf("marshal attributes: %w", err)
	}
	_, err = tx.Exec(`INSERT INTO agents (instance_uid, service_name, namespace, version, node_name, pod_name,
			attributes_json, capabilities, connection_state, last_seen_unix, start_time_unix, healthy,
			last_error, effective_config_yaml, config_sync, last_remote_config_hash, pushed_by, last_pushed_at_unix)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(instance_uid) DO UPDATE SET
			service_name=excluded.service_name, namespace=excluded.namespace, version=excluded.version,
			node_name=excluded.node_name, pod_name=excluded.pod_name, attributes_json=excluded.attributes_json,
			capabilities=excluded.capabilities, connection_state=excluded.connection_state,
			last_seen_unix=excluded.last_seen_unix, start_time_unix=excluded.start_time_unix,
			healthy=excluded.healthy, last_error=excluded.last_error,
			effective_config_yaml=excluded.effective_config_yaml, config_sync=excluded.config_sync,
			last_remote_config_hash=excluded.last_remote_config_hash, pushed_by=excluded.pushed_by,
			last_pushed_at_unix=excluded.last_pushed_at_unix`,
		a.InstanceUID, a.ServiceName, a.Namespace, a.Version, a.NodeName, a.PodName,
		string(attrsJSON), a.Capabilities, string(a.ConnectionState),
		unixSeconds(a.LastSeen), unixSeconds(a.StartTime), boolToInt(a.Healthy),
		a.LastError, a.EffectiveConfigYAML, string(a.ConfigSync), a.LastRemoteConfigHash,
		a.PushedBy, unixSeconds(a.LastPushedAt))
	return err
}

func unixSeconds(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *SQLiteStore) GetAgent(instanceUID string) (Agent, bool) {
	row := s.db.QueryRow(`SELECT instance_uid, service_name, namespace, version, node_name, pod_name,
		attributes_json, capabilities, connection_state, last_seen_unix, start_time_unix, healthy,
		last_error, effective_config_yaml, config_sync, last_remote_config_hash, pushed_by, last_pushed_at_unix
		FROM agents WHERE instance_uid = ?`, instanceUID)
	a, err := scanAgent(row)
	if err != nil {
		return Agent{}, false
	}
	return a, true
}

func (s *SQLiteStore) ListAgents() ([]Agent, error) {
	rows, err := s.db.Query(`SELECT instance_uid, service_name, namespace, version, node_name, pod_name,
		attributes_json, capabilities, connection_state, last_seen_unix, start_time_unix, healthy,
		last_error, effective_config_yaml, config_sync, last_remote_config_hash, pushed_by, last_pushed_at_unix
		FROM agents`)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var out []Agent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, fmt.Errorf("list agents scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) DeleteAgent(instanceUID string) error {
	return s.withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM agents WHERE instance_uid = ?`, instanceUID); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM config_pushes WHERE agent_uid = ?`, instanceUID)
		return err
	})
}

func (s *SQLiteStore) AppendConfigPush(push ConfigPush) error {
	_, err := s.db.Exec(`INSERT INTO config_pushes (id, agent_uid, timestamp_unix, config_yaml, pushed_by, note, succeeded, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		push.ID, push.AgentUID, unixSeconds(push.Timestamp), push.ConfigYAML, push.PushedBy, push.Note,
		boolToInt(push.Succeeded), push.ErrorMessage)
	if err != nil {
		return fmt.Errorf("append config push: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateConfigPushResult(agentUID, pushID string, succeeded bool, errMsg string) error {
	_, err := s.db.Exec(`UPDATE config_pushes SET succeeded = ?, error_message = ? WHERE id = ? AND agent_uid = ?`,
		boolToInt(succeeded), errMsg, pushID, agentUID)
	if err != nil {
		return fmt.Errorf("update config push result: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListConfigPushes(agentUID string) ([]ConfigPush, error) {
	rows, err := s.db.Query(`SELECT id, agent_uid, timestamp_unix, config_yaml, pushed_by, note, succeeded, error_message
		FROM config_pushes WHERE agent_uid = ? ORDER BY timestamp_unix ASC`, agentUID)
	if err != nil {
		return nil, fmt.Errorf("list config pushes: %w", err)
	}
	defer rows.Close()

	var out []ConfigPush
	for rows.Next() {
		var p ConfigPush
		var ts int64
		var succeeded int
		if err := rows.Scan(&p.ID, &p.AgentUID, &ts, &p.ConfigYAML, &p.PushedBy, &p.Note, &succeeded, &p.ErrorMessage); err != nil {
			return nil, fmt.Errorf("list config pushes scan: %w", err)
		}
		p.Timestamp = unixOrZero(ts)
		p.Succeeded = succeeded != 0
		out = append(out, p)
	}
	return out, rows.Err()
}

var (
	_ Store = (*SQLiteStore)(nil)
	_ Store = (*MemoryStore)(nil)
)
