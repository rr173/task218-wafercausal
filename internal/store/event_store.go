package store

import (
	"context"
	"database/sql"
	"errors"

	"task218-wafercausal/internal/model"
)

// EventStore 负责工艺事件的持久化。
type EventStore struct {
	db *sql.DB
}

// InsertEvent 插入工艺事件。
func (s *EventStore) InsertEvent(ctx context.Context, e *model.ProcessEvent) (*model.ProcessEvent, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO process_events (batch_id, step_id, tool, param, occurred_at, status, untrusted, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?);`,
		e.BatchID, e.StepID, e.Tool, e.Param, e.OccurredAt, e.Status, boolToInt(e.Untrusted), e.CreatedAt)
	if err != nil {
		return nil, err
	}
	e.ID, err = res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return e, nil
}

// GetEvent 按 ID 查询工艺事件。
func (s *EventStore) GetEvent(ctx context.Context, id int64) (*model.ProcessEvent, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, batch_id, step_id, tool, param, occurred_at, status, untrusted, created_at
		 FROM process_events WHERE id = ?;`, id)
	var e model.ProcessEvent
	var untrusted int
	err := row.Scan(&e.ID, &e.BatchID, &e.StepID, &e.Tool, &e.Param, &e.OccurredAt,
		&e.Status, &untrusted, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	e.Untrusted = untrusted != 0
	return &e, nil
}

// ListEventsByBatch 列出某批次的全部工艺事件（按发生时间升序）。
func (s *EventStore) ListEventsByBatch(ctx context.Context, batchID int64) ([]*model.ProcessEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, batch_id, step_id, tool, param, occurred_at, status, untrusted, created_at
		 FROM process_events WHERE batch_id = ? ORDER BY occurred_at ASC;`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.ProcessEvent, 0)
	for rows.Next() {
		var e model.ProcessEvent
		var untrusted int
		if err := rows.Scan(&e.ID, &e.BatchID, &e.StepID, &e.Tool, &e.Param, &e.OccurredAt,
			&e.Status, &untrusted, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Untrusted = untrusted != 0
		out = append(out, &e)
	}
	return out, rows.Err()
}

// MarkUntrusted 将工艺事件标记为设备数据不可信（或解除标记）。
func (s *EventStore) MarkUntrusted(ctx context.Context, id int64, untrusted bool) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE process_events SET untrusted = ?, status = ? WHERE id = ?;`,
		boolToInt(untrusted), model.EventValid, id)
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

// UpdateStatus 更新工艺事件状态。
func (s *EventStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE process_events SET status = ? WHERE id = ?;`, status, id)
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

// ListEventsByStep 列出某工艺步骤关联的全部事件。
func (s *EventStore) ListEventsByStep(ctx context.Context, stepID int64) ([]*model.ProcessEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, batch_id, step_id, tool, param, occurred_at, status, untrusted, created_at
		 FROM process_events WHERE step_id = ? ORDER BY occurred_at ASC;`, stepID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.ProcessEvent, 0)
	for rows.Next() {
		var e model.ProcessEvent
		var untrusted int
		if err := rows.Scan(&e.ID, &e.BatchID, &e.StepID, &e.Tool, &e.Param, &e.OccurredAt,
			&e.Status, &untrusted, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Untrusted = untrusted != 0
		out = append(out, &e)
	}
	return out, rows.Err()
}
