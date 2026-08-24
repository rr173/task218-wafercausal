package service

import (
	"context"
	"testing"

	"task218-wafercausal/internal/ingest"
)

func TestTask218Bug09DuplicateDefectImportIsIdempotent(t *testing.T) {
	ctx := context.Background()
	app, st := newTestService(t)
	defer st.Close()
	b, err := app.CreateBatch(ctx, "idempotent", "WFR-218-09", 300)
	if err != nil { t.Fatal(err) }
	input := []ingest.DefectInput{{DetectBatch: "i", X: 1, Y: 1, RadiusUM: 2, DetectedAt: "2026-08-24T00:00:00Z"}}
	first, err := app.ImportDefects(ctx, b.ID, input)
	if err != nil { t.Fatal(err) }
	second, err := app.ImportDefects(ctx, b.ID, input)
	if err != nil { t.Fatal(err) }
	if first.Accepted != 1 || second.Skipped != 1 || second.Accepted != 0 { t.Fatalf("unexpected import results: %+v %+v", first, second) }
	defects, err := app.ListDefects(ctx, b.ID)
	if err != nil { t.Fatal(err) }
	if len(defects) != 1 { t.Fatalf("expected one persisted defect, got %d", len(defects) ) }
}
