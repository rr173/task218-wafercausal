package store

import (
	"context"
	"database/sql"
	"errors"

	"task218-wafercausal/internal/model"
)

// CausalStore 负责因果候选及其路径步骤的持久化。
type CausalStore struct {
	db *sql.DB
}

// CreateCandidate 插入因果候选。
func (s *CausalStore) CreateCandidate(ctx context.Context, c *model.CausalCandidate) (*model.CausalCandidate, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO causal_candidates (batch_id, cluster_id, root_step_id, score, evidence, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?);`,
		c.BatchID, c.ClusterID, c.RootStepID, c.Score, c.Evidence, c.Status, c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.ID, err = res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return c, nil
}

// GetCandidate 按 ID 查询因果候选。
func (s *CausalStore) GetCandidate(ctx context.Context, id int64) (*model.CausalCandidate, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, batch_id, cluster_id, root_step_id, score, evidence, status, created_at
		 FROM causal_candidates WHERE id = ?;`, id)
	var c model.CausalCandidate
	err := row.Scan(&c.ID, &c.BatchID, &c.ClusterID, &c.RootStepID, &c.Score,
		&c.Evidence, &c.Status, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListCandidatesByBatch 列出某批次的全部因果候选。
func (s *CausalStore) ListCandidatesByBatch(ctx context.Context, batchID int64) ([]*model.CausalCandidate, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, batch_id, cluster_id, root_step_id, score, evidence, status, created_at
		 FROM causal_candidates WHERE batch_id = ? ORDER BY id ASC;`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.CausalCandidate, 0)
	for rows.Next() {
		var c model.CausalCandidate
		if err := rows.Scan(&c.ID, &c.BatchID, &c.ClusterID, &c.RootStepID, &c.Score,
			&c.Evidence, &c.Status, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

// ListCandidatesByCluster 列出某空间簇对应的因果候选。
func (s *CausalStore) ListCandidatesByCluster(ctx context.Context, clusterID int64) ([]*model.CausalCandidate, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, batch_id, cluster_id, root_step_id, score, evidence, status, created_at
		 FROM causal_candidates WHERE cluster_id = ? AND status = ? ORDER BY score DESC;`, clusterID, model.CausalStatusCandidate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.CausalCandidate, 0)
	for rows.Next() {
		var c model.CausalCandidate
		if err := rows.Scan(&c.ID, &c.BatchID, &c.ClusterID, &c.RootStepID, &c.Score,
			&c.Evidence, &c.Status, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

// UpdateCandidateStatus 更新因果候选状态。
func (s *CausalStore) UpdateCandidateStatus(ctx context.Context, id int64, status string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE causal_candidates SET status = ? WHERE id = ?;`, status, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return model.ErrNotFound
	}
	return nil
}

// AddCandidateStep 建立因果候选与路径步骤的关联。
func (s *CausalStore) AddCandidateStep(ctx context.Context, candidateID, stepID int64, order int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO candidate_steps (candidate_id, step_id, ord) VALUES (?, ?, ?);`,
		candidateID, stepID, order)
	return err
}

// ListCandidateSteps 列出某候选路径上的步骤（按依赖顺序）。
func (s *CausalStore) ListCandidateSteps(ctx context.Context, candidateID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT step_id FROM candidate_steps WHERE candidate_id = ? ORDER BY ord ASC;`, candidateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
