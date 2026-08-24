package service

import (
	"context"
	"testing"

	"task218-wafercausal/internal/ingest"
	"task218-wafercausal/internal/model"
)

func TestTask218Bug01ArchivedDefectWriteRejected(t *testing.T) {
	ctx := context.Background()
	app, st := newTestService(t)
	defer st.Close()
	b, err := app.CreateBatch(ctx, "archived", "WFR-218-01", 300)
	if err != nil { t.Fatal(err) }
	if _, err := app.ArchiveBatch(ctx, b.ID); err != nil { t.Fatal(err) }
	if _, err := app.ImportDefects(ctx, b.ID, []ingest.DefectInput{{
		DetectBatch: "inspect-1", X: 1, Y: 1, DetectedAt: "2026-08-24T00:00:00Z",
	}}); err != model.ErrArchived {
		t.Fatalf("expected archived batch to reject defect write, got %v", err)
	}
	defects, err := app.ListDefects(ctx, b.ID)
	if err != nil { t.Fatal(err) }
	if len(defects) != 0 { t.Fatalf("archived write persisted %d defects", len(defects)) }
}
