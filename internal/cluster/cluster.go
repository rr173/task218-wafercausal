// Package cluster 实现晶圆缺陷的空间聚类：按坐标邻近把缺陷聚合为空间簇。
package cluster

import (
	"math"

	"task218-wafercausal/internal/model"
)

// ClusterResult 一个空间簇及其归属的缺陷。
type ClusterResult struct {
	Cluster   *model.SpatialCluster
	DefectIDs []int64
}

// Cluster 按欧氏距离贪婪聚类。距离阈值 thresholdUM 单位为微米，
// 缺陷坐标单位为毫米（内部换算 1mm = 1000um）。
// 返回的簇按缺陷数降序排列，方便后续因果分析优先处理高密度簇。
func Cluster(defects []*model.DefectRecord, thresholdUM float64) []ClusterResult {
	if len(defects) == 0 {
		return nil
	}
	type acc struct {
		cx, cy  float64
		n       int
		ids     []int64
		defects []*model.DefectRecord
	}
	var clusters []*acc
	for _, d := range defects {
		best := -1
		bestDist := math.MaxFloat64
		for i, c := range clusters {
			centX := c.cx / float64(c.n)
			centY := c.cy / float64(c.n)
			distUM := distanceMM(d.X, d.Y, centX, centY) * 1000
			if distUM <= thresholdUM && distUM < bestDist {
				best = i
				bestDist = distUM
			}
		}
		if best < 0 {
			clusters = append(clusters, &acc{
				cx: d.X, cy: d.Y, n: 1,
				ids:     []int64{d.ID},
				defects: []*model.DefectRecord{d},
			})
			continue
		}
		c := clusters[best]
		c.cx += d.X
		c.cy += d.Y
		c.n++
		c.ids = append(c.ids, d.ID)
		c.defects = append(c.defects, d)
	}
	results := make([]ClusterResult, 0, len(clusters))
	for _, c := range clusters {
		centX := c.cx / float64(c.n)
		centY := c.cy / float64(c.n)
		results = append(results, ClusterResult{
			Cluster: &model.SpatialCluster{
				CentroidX: centX,
				CentroidY: centY,
				RadiusUM:  RadiusOfDefects(c.defects),
				DefectCnt: c.n,
				Status:    model.ClusterCandidate,
			},
			DefectIDs: c.ids,
		})
	}
	// 按缺陷数降序排序。
	sortResults(results)
	return results
}

// RadiusOfDefects 计算一组缺陷相对其质心的最大半径（微米）。
func RadiusOfDefects(defects []*model.DefectRecord) float64 {
	if len(defects) == 0 {
		return 0
	}
	var sx, sy float64
	for _, d := range defects {
		sx += d.X
		sy += d.Y
	}
	cx, cy := sx/float64(len(defects)), sy/float64(len(defects))
	maxR := 0.0
	for _, d := range defects {
		r := distanceMM(d.X, d.Y, cx, cy) * 1000
		if r > maxR {
			maxR = r
		}
	}
	return maxR
}

// distanceMM 计算两点欧氏距离（毫米）。
func distanceMM(x1, y1, x2, y2 float64) float64 {
	return math.Hypot(x1-x2, y1-y2)
}

// sortResults 按缺陷数降序排序聚类结果。
func sortResults(results []ClusterResult) {
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].Cluster.DefectCnt > results[j-1].Cluster.DefectCnt; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
}
