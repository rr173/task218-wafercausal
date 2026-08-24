package service

import (
	"context"
	"testing"

	"task218-wafercausal/internal/model"
)

func TestTask218Bug04PublishingNewSnapshotSupersedesPrevious(t *testing.T) {
	ctx := context.Background()
	app, st := newTestService(t)
	defer st.Close()
	b, err := app.CreateBatch(ctx, "snapshots", "WFR-218-04", 300)
	if err != nil { t.Fatal(err) }
	first, err := app.PublishSnapshot(ctx, b.ID, "v1")
	if err != nil { t.Fatal(err) }
	second, err := app.PublishSnapshot(ctx, b.ID, "v2")
	if err != nil { t.Fatal(err) }
	if second.Version != 2 { t.Fatalf("expected version 2, got %d", second.Version) }
	got, err := app.ListSnapshots(ctx, b.ID)
	if err != nil { t.Fatal(err) }
	var old, current *model.CausalSnapshot
	for _, sn := range got { if sn.ID == first.ID { old = sn }; if sn.ID == second.ID { current = sn } }
	if old == nil || old.Status != model.SnapshotSuperseded || current == nil || current.Status != model.SnapshotPublished {
		t.Fatalf("unexpected snapshot statuses: old=%+v current=%+v", old, current)
	}
}
