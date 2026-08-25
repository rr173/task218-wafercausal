package store

import (
	"context"
	"database/sql"
	"errors"

	"task218-wafercausal/internal/model"
)

// SnapshotStore 负责因果快照及其候选冻结的持久化。
type SnapshotStore struct {
	db *sql.DB
}

// NextVersion 返回某批次下一个可用快照版本号（1 起递增）。
func (s *SnapshotStore) NextVersion(ctx context.Context, batchID int64) (int, error) {
	var v int
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM snapshots WHERE batch_id = ?;`, batchID).Scan(&v)
	if err != nil {
		return 0, err
	}
	return v + 1, nil
}

// CreateSnapshot 插入因果快照。
func (s *SnapshotStore) CreateSnapshot(ctx context.Context, sn *model.CausalSnapshot) (*model.CausalSnapshot, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO snapshots (batch_id, version, name, status, root_summary, created_at)
		 VALUES (?, ?, ?, ?, ?, ?);`,
		sn.BatchID, sn.Version, sn.Name, sn.Status, sn.RootSummary, sn.CreatedAt)
	if err != nil {
		return nil, err
	}
	sn.ID, err = res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return sn, nil
}

// GetSnapshot 按 ID 查询因果快照。
func (s *SnapshotStore) GetSnapshot(ctx context.Context, id int64) (*model.CausalSnapshot, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, batch_id, version, name, status, root_summary, created_at, superseded_at
		 FROM snapshots WHERE id = ?;`, id)
	var sn model.CausalSnapshot
	err := row.Scan(&sn.ID, &sn.BatchID, &sn.Version, &sn.Name, &sn.Status,
		&sn.RootSummary, &sn.CreatedAt, &sn.SupersededAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sn, nil
}

// ListSnapshotsByBatch 列出某批次的全部快照（按版本降序）。
func (s *SnapshotStore) ListSnapshotsByBatch(ctx context.Context, batchID int64) ([]*model.CausalSnapshot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, batch_id, version, name, status, root_summary, created_at, superseded_at
		 FROM snapshots WHERE batch_id = ? ORDER BY version DESC;`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.CausalSnapshot, 0)
	for rows.Next() {
		var sn model.CausalSnapshot
		if err := rows.Scan(&sn.ID, &sn.BatchID, &sn.Version, &sn.Name, &sn.Status,
			&sn.RootSummary, &sn.CreatedAt, &sn.SupersededAt); err != nil {
			return nil, err
		}
		out = append(out, &sn)
	}
	return out, rows.Err()
}

// Supersede 将某批次早于给定版本的所有已发布快照标记为替代。
func (s *SnapshotStore) Supersede(ctx context.Context, batchID int64, exceptVersion int, supersededAt string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE snapshots SET status = ?, superseded_at = ?
		 WHERE batch_id = ? AND version < ? AND status = ?;`,
		model.SnapshotSuperseded, supersededAt, batchID, exceptVersion, model.SnapshotPublished)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// AddSnapshotCandidate 冻结快照与候选的关联，记录该候选在发布时刻的裁决结果。
// decision 必须为该候选当时实际的状态（candidate / confirmed_root / excluded / superseded），
// 落盘后即不可变证据，调用方负责冻结正确状态，本方法不得改写其语义。
func (s *SnapshotStore) AddSnapshotCandidate(ctx context.Context, snapshotID, candidateID int64, decision string) error {
	if decision == "" {
		decision = model.CausalStatusCandidate
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO snapshot_candidates (snapshot_id, candidate_id, decision) VALUES (?, ?, ?);`,
		snapshotID, candidateID, decision)
	return err
}

// ListSnapshotCandidates 列出某快照冻结的候选关联。
func (s *SnapshotStore) ListSnapshotCandidates(ctx context.Context, snapshotID int64) ([]*model.SnapshotCandidate, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT snapshot_id, candidate_id, decision FROM snapshot_candidates WHERE snapshot_id = ? ORDER BY candidate_id ASC;`,
		snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.SnapshotCandidate, 0)
	for rows.Next() {
		var sc model.SnapshotCandidate
		if err := rows.Scan(&sc.SnapshotID, &sc.CandidateID, &sc.Decision); err != nil {
			return nil, err
		}
		out = append(out, &sc)
	}
	return out, rows.Err()
}
