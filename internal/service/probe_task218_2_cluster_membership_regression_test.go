package service

import (
	"context"
	"testing"

	"task218-wafercausal/internal/ingest"
)

func TestTask218Bug02ClusterKeepsEveryDefectMembership(t *testing.T) {
	ctx := context.Background()
	app, st := newTestService(t)
	defer st.Close()
	b, err := app.CreateBatch(ctx, "cluster", "WFR-218-02", 300)
	if err != nil { t.Fatal(err) }
	if _, err := app.ImportDefects(ctx, b.ID, []ingest.DefectInput{
		{DetectBatch: "i", X: 1, Y: 1, DetectedAt: "2026-08-24T00:00:00Z"},
		{DetectBatch: "i", X: 1.1, Y: 1, DetectedAt: "2026-08-24T00:01:00Z"},
		{DetectBatch: "i", X: 1.2, Y: 1, DetectedAt: "2026-08-24T00:02:00Z"},
	}); err != nil { t.Fatal(err) }
	if _, err := app.RunClustering(ctx, b.ID, 5000); err != nil { t.Fatal(err) }
	clusters, err := app.ListClusters(ctx, b.ID)
	if err != nil { t.Fatal(err) }
	if len(clusters) != 1 { t.Fatalf("expected one cluster, got %d", len(clusters)) }
	ids, err := app.Repos().Clusters.ListClusterDefectIDs(ctx, clusters[0].ID)
	if err != nil { t.Fatal(err) }
	if len(ids) != 3 { t.Fatalf("expected all 3 defects in cluster, got %v", ids) }
}
