package store

import (
	"context"
	"fmt"
	"testing"

	"task218-wafercausal/internal/model"
)

func TestBatchRoundTrip(t *testing.T) {
	ctx := context.Background()
	st, err := Open("")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	repos := NewRepositories(st)

	b, err := repos.Batches.CreateBatch(ctx, &model.WaferBatch{
		Name: "w1", WaferID: "WFR-1", DiameterMM: 300,
		Status: model.BatchProducing, CreatedAt: model.NowStr(), UpdatedAt: model.NowStr(),
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	got, err := repos.Batches.GetBatch(ctx, b.ID)
	if err != nil {
		t.Fatalf("get batch: %v", err)
	}
	if got.WaferID != "WFR-1" || got.Status != model.BatchProducing {
		t.Fatalf("unexpected batch: %+v", got)
	}
	if _, err := repos.Batches.GetBatch(ctx, 99999); err != model.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDefectIdempotency(t *testing.T) {
	ctx := context.Background()
	st, err := Open("")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	repos := NewRepositories(st)

	b, _ := repos.Batches.CreateBatch(ctx, &model.WaferBatch{
		Name: "w1", WaferID: "WFR-1", DiameterMM: 300,
		Status: model.BatchProducing, CreatedAt: model.NowStr(), UpdatedAt: model.NowStr(),
	})
	d := &model.DefectRecord{
		BatchID: b.ID, DetectBatch: "i1", X: 1, Y: 1,
		Severity: "major", DetectedAt: "2026-08-23T08:00:00Z",
		Fingerprint: "fp-1", Status: model.DefectNew, CreatedAt: model.NowStr(),
	}
	if _, err := repos.Defects.InsertDefect(ctx, d); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// 同指纹重复插入应返回 ErrDuplicate。
	d2 := *d
	if _, err := repos.Defects.InsertDefect(ctx, &d2); err != model.ErrDuplicate {
		t.Fatalf("expected ErrDuplicate, got %v", err)
	}
	n, err := repos.Defects.CountByBatch(ctx, b.ID)
	if err != nil || n != 1 {
		t.Fatalf("expected 1 defect, got %d %v", n, err)
	}
}

func TestStatsAndIntegrity(t *testing.T) {
	ctx := context.Background()
	st, err := Open("")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	repos := NewRepositories(st)
	stats, err := repos.Stats(ctx)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats["integrity_check"] != 1 {
		t.Fatalf("integrity check should pass, got %+v", stats)
	}
}

// TestClusterMembershipCoversEveryDefect 断言空间簇与缺陷的归属关系完整覆盖本批次
// 的每条缺陷记录：写入多少成员就能读回多少，禁止静默丢弃首尾成员。
func TestClusterMembershipCoversEveryDefect(t *testing.T) {
	ctx := context.Background()
	st, err := Open("")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	repos := NewRepositories(st)

	b, err := repos.Batches.CreateBatch(ctx, &model.WaferBatch{
		Name: "w1", WaferID: "WFR-MEM", DiameterMM: 300,
		Status: model.BatchProducing, CreatedAt: model.NowStr(), UpdatedAt: model.NowStr(),
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	// 写入 5 条缺陷。
	var defectIDs []int64
	for i, x := range []float64{0, 0.1, 0.2, 80, 80.1} {
		d, err := repos.Defects.InsertDefect(ctx, &model.DefectRecord{
			BatchID: b.ID, DetectBatch: "i1", X: x, Y: x,
			Severity: "major", DetectedAt: "2026-08-23T08:00:00Z",
			Fingerprint: fmt.Sprintf("fp-mem-%d", i), Status: model.DefectNew, CreatedAt: model.NowStr(),
		})
		if err != nil {
			t.Fatalf("insert defect %d: %v", i, err)
		}
		defectIDs = append(defectIDs, d.ID)
	}

	// 单簇：全部 5 条归属同一簇。
	cl, err := repos.Clusters.CreateCluster(ctx, &model.SpatialCluster{
		BatchID: b.ID, CentroidX: 0, CentroidY: 0, RadiusUM: 5000,
		DefectCnt: len(defectIDs), Status: model.ClusterCandidate, CreatedAt: model.NowStr(),
	})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	for _, did := range defectIDs {
		if err := repos.Clusters.AddClusterDefect(ctx, cl.ID, did); err != nil {
			t.Fatalf("add cluster defect %d: %v", did, err)
		}
	}

	// 读回的全部成员必须等于写入的全部，不丢首尾。
	gotIDs, err := repos.Clusters.ListClusterDefectIDs(ctx, cl.ID)
	if err != nil {
		t.Fatalf("list cluster defect ids: %v", err)
	}
	if len(gotIDs) != len(defectIDs) {
		t.Fatalf("expected %d members, got %d", len(defectIDs), len(gotIDs))
	}
	gotSet := make(map[int64]bool, len(gotIDs))
	for _, id := range gotIDs {
		gotSet[id] = true
	}
	for _, id := range defectIDs {
		if !gotSet[id] {
			t.Fatalf("defect %d silently dropped from cluster membership", id)
		}
	}

	// 按簇列缺陷也应返回全部成员。
	defects, err := repos.Defects.ListDefectsByCluster(ctx, cl.ID)
	if err != nil {
		t.Fatalf("list defects by cluster: %v", err)
	}
	if len(defects) != len(defectIDs) {
		t.Fatalf("expected %d defects by cluster, got %d", len(defectIDs), len(defects))
	}
}
