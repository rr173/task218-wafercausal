package causal

import (
	"context"
	"testing"
	"time"

	"task218-wafercausal/internal/model"
	"task218-wafercausal/internal/store"
)

// newTestGenerator 构造一个挂载临时数据库的生成器。
func newTestGenerator(t *testing.T) (*Generator, *store.Store) {
	t.Helper()
	st, err := store.Open("")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return NewGenerator(store.NewRepositories(st)), st
}

// TestGenerateClusterWithoutEvents 回归测试：当一个空间簇对应的工艺步骤没有任何
// 工艺事件时，因果分析不应 panic，而应返回低置信度候选。
// 复现原始 panic: runtime error: index out of range [0] with length 0
// 调用链 scoreStep → generateForCluster → GenerateCandidates。
func TestGenerateClusterWithoutEvents(t *testing.T) {
	ctx := context.Background()
	gen, st := newTestGenerator(t)
	defer st.Close()
	repos := gen.repos

	// 建批次（直接落盘，跳过服务层校验，聚焦因果生成逻辑）。
	batch := &model.WaferBatch{Name: "no-events", WaferID: "WFR-NE", DiameterMM: 300,
		Status: model.BatchPending, CreatedAt: model.NowStr(), UpdatedAt: model.NowStr()}
	b, err := repos.Batches.CreateBatch(ctx, batch)
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	// 工艺步骤链：有步骤但无任何工艺事件。
	steps := []*model.ProcessStep{
		{BatchID: b.ID, Name: "litho", Sequence: 1, CreatedAt: model.NowStr()},
		{BatchID: b.ID, Name: "etch", Sequence: 2, CreatedAt: model.NowStr()},
		{BatchID: b.ID, Name: "deposition", Sequence: 3, CreatedAt: model.NowStr()},
	}
	for i, s := range steps {
		created, err := repos.Steps.CreateStep(ctx, s)
		if err != nil {
			t.Fatalf("create step %d: %v", i, err)
		}
		steps[i] = created
	}

	// 一个空间簇 + 关联缺陷（缺陷用于推算参考时间）。
	defect := &model.DefectRecord{BatchID: b.ID, DetectBatch: "i1", X: 1, Y: 1,
		Severity: "major", DetectedAt: time.Now().UTC().Format(time.RFC3339),
		Status: model.DefectClustered, CreatedAt: model.NowStr()}
	d, err := repos.Defects.InsertDefect(ctx, defect)
	if err != nil {
		t.Fatalf("create defect: %v", err)
	}
	cluster := &model.SpatialCluster{BatchID: b.ID, CentroidX: 1, CentroidY: 1, RadiusUM: 2,
		DefectCnt: 1, Status: model.ClusterCandidate, CreatedAt: model.NowStr()}
	cl, err := repos.Clusters.CreateCluster(ctx, cluster)
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if err := repos.Clusters.AddClusterDefect(ctx, cl.ID, d.ID); err != nil {
		t.Fatalf("link defect: %v", err)
	}

	// 此前会触发 panic；修复后应返回低置信度候选而非崩溃。
	specs, err := gen.Generate(ctx, b.ID)
	if err != nil {
		t.Fatalf("Generate returned error (expected graceful low-confidence candidates): %v", err)
	}
	if len(specs) == 0 {
		t.Fatalf("expected low-confidence candidates, got none")
	}
	for _, sp := range specs {
		if sp.Score > 0.1 {
			t.Fatalf("expected low-confidence score for missing events, got %v", sp.Score)
		}
	}
}
