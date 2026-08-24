package store

import (
	"context"
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
