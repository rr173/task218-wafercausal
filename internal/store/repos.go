// Package store 汇总各仓储，供 service 层统一访问。
package store

import (
	"context"

	"task218-wafercausal/internal/model"
)

// Repositories 聚合全部仓储。
type Repositories struct {
	Store     *Store
	Batches   *BatchStore
	Steps     *StepStore
	Defects   *DefectStore
	Events    *EventStore
	Clusters  *ClusterStore
	Causals   *CausalStore
	Rulings   *RulingStore
	Snapshots *SnapshotStore
}

// NewRepositories 从 Store 构造全部仓储。
func NewRepositories(st *Store) *Repositories {
	return &Repositories{
		Store:     st,
		Batches:   &BatchStore{db: st.DB()},
		Steps:     &StepStore{db: st.DB()},
		Defects:   &DefectStore{db: st.DB()},
		Events:    &EventStore{db: st.DB()},
		Clusters:  &ClusterStore{db: st.DB()},
		Causals:   &CausalStore{db: st.DB()},
		Rulings:   &RulingStore{db: st.DB()},
		Snapshots: &SnapshotStore{db: st.DB()},
	}
}

// AuditEvent 记录审计事件（写操作留痕）。
func (r *Repositories) AuditEvent(ctx context.Context, actor, action, subject, detail string) error {
	_, err := r.Store.DB().ExecContext(ctx,
		`INSERT INTO audit_events (actor, action, subject, detail, created_at)
		 VALUES (?, ?, ?, ?, ?);`,
		actor, action, subject, detail, model.NowStr())
	return err
}

// ListAuditEvents 列出审计事件（倒序）。
func (r *Repositories) ListAuditEvents(ctx context.Context, limit int) ([]map[string]string, error) {
	rows, err := r.Store.DB().QueryContext(ctx,
		`SELECT id, actor, action, subject, detail, created_at
		 FROM audit_events ORDER BY id DESC LIMIT ?;`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]string, 0)
	for rows.Next() {
		var id int64
		var actor, action, subject, detail, created string
		if err := rows.Scan(&id, &actor, &action, &subject, &detail, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]string{
			"id": itoa(id), "actor": actor, "action": action,
			"subject": subject, "detail": detail, "created_at": created,
		})
	}
	return out, rows.Err()
}

// Stats 返回数据库关键统计。
func (r *Repositories) Stats(ctx context.Context) (map[string]int64, error) {
	tables := []string{"wafer_batches", "process_steps", "defect_records",
		"process_events", "spatial_clusters", "cluster_defects",
		"causal_candidates", "candidate_steps", "evidence_rulings",
		"snapshots", "snapshot_candidates"}
	out := make(map[string]int64, len(tables))
	for _, t := range tables {
		n, err := r.Store.TableCount(t)
		if err != nil {
			return nil, err
		}
		out[t] = n
	}
	ok, err := r.Store.IntegrityCheck()
	if err != nil {
		return nil, err
	}
	if ok {
		out["integrity_check"] = 1
	} else {
		out["integrity_check"] = 0
	}
	return out, nil
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
