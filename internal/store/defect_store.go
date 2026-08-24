package store

import (
	"context"
	"database/sql"
	"errors"

	"task218-wafercausal/internal/model"
)

// DefectStore 负责缺陷记录的持久化。
type DefectStore struct {
	db *sql.DB
}

// InsertDefect 幂等插入缺陷记录：同批次内 fingerprint 重复时返回 model.ErrDuplicate。
func (s *DefectStore) InsertDefect(ctx context.Context, d *model.DefectRecord) (*model.DefectRecord, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO defect_records
		 (batch_id, detect_batch, x, y, radius_um, severity, detected_at, fingerprint, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		d.BatchID, d.DetectBatch, d.X, d.Y, d.RadiusUM, d.Severity, d.DetectedAt,
		d.Fingerprint, d.Status, d.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, model.ErrDuplicate
		}
		return nil, err
	}
	d.ID, err = res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return d, nil
}

// GetDefect 按 ID 查询缺陷记录。
func (s *DefectStore) GetDefect(ctx context.Context, id int64) (*model.DefectRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, batch_id, detect_batch, x, y, radius_um, severity, detected_at, fingerprint, status, created_at
		 FROM defect_records WHERE id = ?;`, id)
	var d model.DefectRecord
	err := row.Scan(&d.ID, &d.BatchID, &d.DetectBatch, &d.X, &d.Y, &d.RadiusUM,
		&d.Severity, &d.DetectedAt, &d.Fingerprint, &d.Status, &d.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ListDefectsByBatch 列出某批次的全部缺陷记录。
func (s *DefectStore) ListDefectsByBatch(ctx context.Context, batchID int64) ([]*model.DefectRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, batch_id, detect_batch, x, y, radius_um, severity, detected_at, fingerprint, status, created_at
		 FROM defect_records WHERE batch_id = ? ORDER BY id ASC;`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.DefectRecord, 0)
	for rows.Next() {
		var d model.DefectRecord
		if err := rows.Scan(&d.ID, &d.BatchID, &d.DetectBatch, &d.X, &d.Y, &d.RadiusUM,
			&d.Severity, &d.DetectedAt, &d.Fingerprint, &d.Status, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}

// UpdateStatus 更新缺陷状态。
func (s *DefectStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE defect_records SET status = ? WHERE id = ?;`, status, id)
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

// ListDefectsByCluster 列出某空间簇归属的全部缺陷。
func (s *DefectStore) ListDefectsByCluster(ctx context.Context, clusterID int64) ([]*model.DefectRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT d.id, d.batch_id, d.detect_batch, d.x, d.y, d.radius_um, d.severity,
		        d.detected_at, d.fingerprint, d.status, d.created_at
		 FROM defect_records d
		 JOIN cluster_defects cd ON cd.defect_id = d.id
		 WHERE cd.cluster_id = ? ORDER BY d.id ASC;`, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.DefectRecord, 0)
	for rows.Next() {
		var d model.DefectRecord
		if err := rows.Scan(&d.ID, &d.BatchID, &d.DetectBatch, &d.X, &d.Y, &d.RadiusUM,
			&d.Severity, &d.DetectedAt, &d.Fingerprint, &d.Status, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}

// CountByBatch 统计某批次的缺陷数。
func (s *DefectStore) CountByBatch(ctx context.Context, batchID int64) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM defect_records WHERE batch_id = ?;`, batchID).Scan(&n)
	return n, err
}
