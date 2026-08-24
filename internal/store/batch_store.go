package store

import (
	"context"
	"database/sql"
	"errors"

	"task218-wafercausal/internal/model"
)

// BatchStore 负责晶圆批次的持久化。
type BatchStore struct {
	db *sql.DB
}

// CreateBatch 插入新晶圆批次。
func (s *BatchStore) CreateBatch(ctx context.Context, b *model.WaferBatch) (*model.WaferBatch, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO wafer_batches (name, wafer_id, diameter_mm, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?);`,
		b.Name, b.WaferID, b.DiameterMM, b.Status, b.CreatedAt, b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	b.ID, err = res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return b, nil
}

// GetBatch 按 ID 查询晶圆批次。
func (s *BatchStore) GetBatch(ctx context.Context, id int64) (*model.WaferBatch, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, wafer_id, diameter_mm, status, created_at, updated_at, archived_at
		 FROM wafer_batches WHERE id = ?;`, id)
	var b model.WaferBatch
	err := row.Scan(&b.ID, &b.Name, &b.WaferID, &b.DiameterMM, &b.Status,
		&b.CreatedAt, &b.UpdatedAt, &b.ArchivedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// ListBatches 列出全部晶圆批次（按创建时间倒序）。
func (s *BatchStore) ListBatches(ctx context.Context) ([]*model.WaferBatch, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, wafer_id, diameter_mm, status, created_at, updated_at, archived_at
		 FROM wafer_batches ORDER BY id DESC;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.WaferBatch, 0)
	for rows.Next() {
		var b model.WaferBatch
		if err := rows.Scan(&b.ID, &b.Name, &b.WaferID, &b.DiameterMM, &b.Status,
			&b.CreatedAt, &b.UpdatedAt, &b.ArchivedAt); err != nil {
			return nil, err
		}
		out = append(out, &b)
	}
	return out, rows.Err()
}

// UpdateStatus 更新批次状态（附更新时间戳）。
func (s *BatchStore) UpdateStatus(ctx context.Context, id int64, status, updatedAt string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE wafer_batches SET status = ?, updated_at = ? WHERE id = ?;`,
		status, updatedAt, id)
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

// Archive 封存批次（写入封存时间）。
func (s *BatchStore) Archive(ctx context.Context, id int64, archivedAt, updatedAt string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE wafer_batches SET status = ?, archived_at = ?, updated_at = ? WHERE id = ?;`,
		model.BatchArchived, archivedAt, updatedAt, id)
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
