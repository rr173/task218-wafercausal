package store

import (
	"context"
	"database/sql"
	"errors"

	"task218-wafercausal/internal/model"
)

// ClusterStore 负责空间簇及其缺陷归属的持久化。
type ClusterStore struct {
	db *sql.DB
}

// CreateCluster 插入空间簇。
func (s *ClusterStore) CreateCluster(ctx context.Context, c *model.SpatialCluster) (*model.SpatialCluster, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO spatial_clusters (batch_id, centroid_x, centroid_y, radius_um, defect_cnt, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?);`,
		c.BatchID, c.CentroidX, c.CentroidY, c.RadiusUM, c.DefectCnt, c.Status, c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.ID, err = res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return c, nil
}

// GetCluster 按 ID 查询空间簇。
func (s *ClusterStore) GetCluster(ctx context.Context, id int64) (*model.SpatialCluster, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, batch_id, centroid_x, centroid_y, radius_um, defect_cnt, status, created_at
		 FROM spatial_clusters WHERE id = ?;`, id)
	var c model.SpatialCluster
	err := row.Scan(&c.ID, &c.BatchID, &c.CentroidX, &c.CentroidY, &c.RadiusUM,
		&c.DefectCnt, &c.Status, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListClustersByBatch 列出某批次的全部空间簇。
func (s *ClusterStore) ListClustersByBatch(ctx context.Context, batchID int64) ([]*model.SpatialCluster, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, batch_id, centroid_x, centroid_y, radius_um, defect_cnt, status, created_at
		 FROM spatial_clusters WHERE batch_id = ? ORDER BY id ASC;`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.SpatialCluster, 0)
	for rows.Next() {
		var c model.SpatialCluster
		if err := rows.Scan(&c.ID, &c.BatchID, &c.CentroidX, &c.CentroidY, &c.RadiusUM,
			&c.DefectCnt, &c.Status, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

// UpdateStatus 更新空间簇状态。
func (s *ClusterStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE spatial_clusters SET status = ? WHERE id = ?;`, status, id)
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

// AddClusterDefect 建立空间簇与缺陷的归属关系（幂等）。
func (s *ClusterStore) AddClusterDefect(ctx context.Context, clusterID, defectID int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO cluster_defects (cluster_id, defect_id) VALUES (?, ?);`,
		clusterID, defectID)
	return err
}

// ListClusterDefectIDs 列出某空间簇归属的全部缺陷 ID。
func (s *ClusterStore) ListClusterDefectIDs(ctx context.Context, clusterID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT defect_id FROM cluster_defects WHERE cluster_id = ? ORDER BY defect_id ASC;`, clusterID)
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

// DeleteClustersByBatch 删除某批次全部空间簇（重新聚类前清理旧簇）。
func (s *ClusterStore) DeleteClustersByBatch(ctx context.Context, batchID int64) (int64, error) {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM cluster_defects WHERE cluster_id IN (
			SELECT id FROM spatial_clusters WHERE batch_id = ?
		);`, batchID); err != nil {
		return 0, err
	}
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM spatial_clusters WHERE batch_id = ?;`, batchID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
