package cluster

import (
	"testing"

	"task218-wafercausal/internal/model"
)

func def(id int64, x, y float64) *model.DefectRecord {
	return &model.DefectRecord{ID: id, X: x, Y: y}
}

func TestClusterGroupsNearbyDefects(t *testing.T) {
	defects := []*model.DefectRecord{
		def(1, 0, 0),
		def(2, 0.1, 0.1),
		def(3, 0.2, -0.1),
		def(4, 80, 80),
		def(5, 80.1, 80.2),
	}
	results := Cluster(defects, 5000) // 阈值 5mm
	if len(results) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(results))
	}
	// 高密度簇应排前面（3 个缺陷）。
	if results[0].Cluster.DefectCnt != 3 {
		t.Fatalf("expected first cluster to have 3 defects, got %d", results[0].Cluster.DefectCnt)
	}
	if results[1].Cluster.DefectCnt != 2 {
		t.Fatalf("expected second cluster to have 2 defects, got %d", results[1].Cluster.DefectCnt)
	}
}

func TestClusterEmpty(t *testing.T) {
	if got := Cluster(nil, 5000); got != nil {
		t.Fatalf("expected nil for empty input, got %v", got)
	}
}

// TestClusterCoversEveryDefect 断言一次聚类的归属结果必须覆盖输入的每条缺陷，
// 且每簇 DefectIDs 数量与 Cluster.DefectCnt 一致，禁止静默丢弃成员。
func TestClusterCoversEveryDefect(t *testing.T) {
	defects := []*model.DefectRecord{
		def(1, 0, 0),
		def(2, 0.1, 0.1),
		def(3, 0.2, -0.1),
		def(4, 80, 80),
		def(5, 80.1, 80.2),
	}
	results := Cluster(defects, 5000)

	want := map[int64]bool{1: true, 2: true, 3: true, 4: true, 5: true}
	got := make(map[int64]bool)
	var totalMembers int
	for _, r := range results {
		if len(r.DefectIDs) != r.Cluster.DefectCnt {
			t.Fatalf("cluster centroid (%.2f,%.2f): DefectIDs=%d but DefectCnt=%d",
				r.Cluster.CentroidX, r.Cluster.CentroidY, len(r.DefectIDs), r.Cluster.DefectCnt)
		}
		totalMembers += len(r.DefectIDs)
		for _, id := range r.DefectIDs {
			got[id] = true
		}
	}
	if totalMembers != len(defects) {
		t.Fatalf("expected %d total members, got %d", len(defects), totalMembers)
	}
	for id := range want {
		if !got[id] {
			t.Fatalf("defect %d silently dropped from cluster membership", id)
		}
	}
}

// TestClusterSingleDefectPreserved 断言单缺陷簇也必须保留其唯一的成员，
// 不被截断为空归属。
func TestClusterSingleDefectPreserved(t *testing.T) {
	defects := []*model.DefectRecord{def(7, 0, 0), def(8, 90, 90)}
	results := Cluster(defects, 5000)
	if len(results) != 2 {
		t.Fatalf("expected 2 single-defect clusters, got %d", len(results))
	}
	var totalMembers int
	for _, r := range results {
		if len(r.DefectIDs) != 1 {
			t.Fatalf("single-defect cluster must keep its 1 member, got %d", len(r.DefectIDs))
		}
		totalMembers += len(r.DefectIDs)
	}
	if totalMembers != len(defects) {
		t.Fatalf("expected %d total members, got %d", len(defects), totalMembers)
	}
}

func TestRadiusOfDefects(t *testing.T) {
	defects := []*model.DefectRecord{
		def(1, 0, 0),
		def(2, 3, 4), // 两点距离 5mm，质心在 (1.5,2)，半径 2.5mm = 2500um
	}
	r := RadiusOfDefects(defects)
	if r < 2400 || r > 2600 {
		t.Fatalf("expected radius around 2500um, got %.2f", r)
	}
}
