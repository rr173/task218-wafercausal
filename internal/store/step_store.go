package store

import (
	"context"
	"database/sql"
	"errors"

	"task218-wafercausal/internal/model"
)

// StepStore 负责工艺步骤的持久化。
type StepStore struct {
	db *sql.DB
}

// CreateStep 插入工艺步骤。
func (s *StepStore) CreateStep(ctx context.Context, st *model.ProcessStep) (*model.ProcessStep, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO process_steps (batch_id, name, seq, parent_step_id, tool, created_at)
		 VALUES (?, ?, ?, ?, ?, ?);`,
		st.BatchID, st.Name, st.Sequence, st.ParentStepID, st.Tool, st.CreatedAt)
	if err != nil {
		return nil, err
	}
	st.ID, err = res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return st, nil
}

// GetStep 按 ID 查询工艺步骤。
func (s *StepStore) GetStep(ctx context.Context, id int64) (*model.ProcessStep, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, batch_id, name, seq, parent_step_id, tool, created_at
		 FROM process_steps WHERE id = ?;`, id)
	var st model.ProcessStep
	err := row.Scan(&st.ID, &st.BatchID, &st.Name, &st.Sequence, &st.ParentStepID,
		&st.Tool, &st.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// ListStepsByBatch 列出某批次的全部工艺步骤（按执行顺序）。
func (s *StepStore) ListStepsByBatch(ctx context.Context, batchID int64) ([]*model.ProcessStep, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, batch_id, name, seq, parent_step_id, tool, created_at
		 FROM process_steps WHERE batch_id = ? ORDER BY seq DESC;`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.ProcessStep, 0)
	for rows.Next() {
		var st model.ProcessStep
		if err := rows.Scan(&st.ID, &st.BatchID, &st.Name, &st.Sequence, &st.ParentStepID,
			&st.Tool, &st.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &st)
	}
	return out, rows.Err()
}

// SetParent 更新工艺步骤的前置依赖。
func (s *StepStore) SetParent(ctx context.Context, id int64, parentStepID int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE process_steps SET parent_step_id = ? WHERE id = ?;`, parentStepID, id)
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

// CountByBatch 统计某批次的工艺步骤数。
func (s *StepStore) CountByBatch(ctx context.Context, batchID int64) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM process_steps WHERE batch_id = ?;`, batchID).Scan(&n)
	return n, err
}
