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
