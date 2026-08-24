// Package store 提供基于 SQLite（modernc.org/sqlite，纯 Go 驱动）的持久化能力。
// 所有业务实体（晶圆批次、工艺步骤、缺陷记录、工艺事件、空间簇、因果候选、
// 证据裁决、因果快照）均落盘，服务重启后可通过数据库完整恢复运行状态。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store 封装 SQLite 连接与建表迁移。
type Store struct {
	db   *sql.DB
	path string
}

// Path 返回数据库文件路径。
func (s *Store) Path() string { return s.path }

// Open 打开（或创建）位于 path 的 SQLite 数据库并执行建表迁移。
// path 为空时使用临时文件，便于测试与自检。
func Open(path string) (*Store, error) {
	if path == "" {
		dir, err := os.MkdirTemp("", "wafercausal-*")
		if err != nil {
			return nil, err
		}
		path = filepath.Join(dir, "wafercausal.db")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		return nil, fmt.Errorf("enable wal: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON;`); err != nil {
		return nil, fmt.Errorf("enable fk: %w", err)
	}
	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// DB 返回底层连接，供事务与仓储使用。
func (s *Store) DB() *sql.DB { return s.db }

// Close 关闭数据库。
func (s *Store) Close() error { return s.db.Close() }

// migrate 按依赖顺序建表。业务数据全部落盘，具备重启恢复路径。
func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS wafer_batches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			wafer_id TEXT NOT NULL,
			diameter_mm REAL NOT NULL DEFAULT 300,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			archived_at TEXT NOT NULL DEFAULT '',
			UNIQUE(wafer_id)
		);`,
		`CREATE TABLE IF NOT EXISTS process_steps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES wafer_batches(id),
			name TEXT NOT NULL,
			seq INTEGER NOT NULL DEFAULT 0,
			parent_step_id INTEGER NOT NULL DEFAULT 0,
			tool TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			UNIQUE(batch_id, name)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_steps_batch ON process_steps(batch_id);`,
		`CREATE TABLE IF NOT EXISTS defect_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES wafer_batches(id),
			detect_batch TEXT NOT NULL,
			x REAL NOT NULL,
			y REAL NOT NULL,
			radius_um REAL NOT NULL DEFAULT 0,
			severity TEXT NOT NULL DEFAULT 'minor',
			detected_at TEXT NOT NULL,
			fingerprint TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(batch_id, fingerprint)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_defects_batch ON defect_records(batch_id);`,
		`CREATE TABLE IF NOT EXISTS process_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES wafer_batches(id),
			step_id INTEGER NOT NULL REFERENCES process_steps(id),
			tool TEXT NOT NULL DEFAULT '',
			param TEXT NOT NULL DEFAULT '{}',
			occurred_at TEXT NOT NULL,
			status TEXT NOT NULL,
			untrusted INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_events_batch ON process_events(batch_id);`,
		`CREATE INDEX IF NOT EXISTS idx_events_step ON process_events(step_id);`,
		`CREATE TABLE IF NOT EXISTS spatial_clusters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES wafer_batches(id),
			centroid_x REAL NOT NULL,
			centroid_y REAL NOT NULL,
			radius_um REAL NOT NULL DEFAULT 0,
			defect_cnt INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_clusters_batch ON spatial_clusters(batch_id);`,
		`CREATE TABLE IF NOT EXISTS cluster_defects (
			cluster_id INTEGER NOT NULL REFERENCES spatial_clusters(id),
			defect_id INTEGER NOT NULL REFERENCES defect_records(id),
			PRIMARY KEY (cluster_id, defect_id)
		);`,
		`CREATE TABLE IF NOT EXISTS causal_candidates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES wafer_batches(id),
			cluster_id INTEGER NOT NULL REFERENCES spatial_clusters(id),
			root_step_id INTEGER NOT NULL DEFAULT 0,
			score REAL NOT NULL DEFAULT 0,
			evidence TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_candidates_batch ON causal_candidates(batch_id);`,
		`CREATE TABLE IF NOT EXISTS candidate_steps (
			candidate_id INTEGER NOT NULL REFERENCES causal_candidates(id),
			step_id INTEGER NOT NULL REFERENCES process_steps(id),
			ord INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (candidate_id, step_id)
		);`,
		`CREATE TABLE IF NOT EXISTS evidence_rulings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES wafer_batches(id),
			candidate_id INTEGER NOT NULL REFERENCES causal_candidates(id),
			decision TEXT NOT NULL,
			reason TEXT NOT NULL DEFAULT '',
			actor TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_rulings_batch ON evidence_rulings(batch_id);`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES wafer_batches(id),
			version INTEGER NOT NULL,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			root_summary TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			superseded_at TEXT NOT NULL DEFAULT '',
			UNIQUE(batch_id, version)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_batch ON snapshots(batch_id);`,
		`CREATE TABLE IF NOT EXISTS snapshot_candidates (
			snapshot_id INTEGER NOT NULL REFERENCES snapshots(id),
			candidate_id INTEGER NOT NULL REFERENCES causal_candidates(id),
			decision TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (snapshot_id, candidate_id)
		);`,
		`CREATE TABLE IF NOT EXISTS audit_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor TEXT NOT NULL,
			action TEXT NOT NULL,
			subject TEXT NOT NULL,
			detail TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// TableCount 返回某张表的总行数（自检统计用）。
func (s *Store) TableCount(table string) (int64, error) {
	var n int64
	err := s.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s;`, table)).Scan(&n)
	return n, err
}

// IntegrityCheck 执行 SQLite 完整性检查，返回是否通过。
func (s *Store) IntegrityCheck() (bool, error) {
	var res string
	if err := s.db.QueryRow(`PRAGMA integrity_check;`).Scan(&res); err != nil {
		return false, err
	}
	return res == "ok", nil
}
