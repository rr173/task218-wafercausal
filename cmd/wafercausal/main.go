// 芯片晶圆缺陷因果链追踪服务入口。
//
// 用法：
//
//	wafercausal --addr :8080 --db wafercausal.db   # 启动 HTTP 服务
//	wafercausal --smoke-test                       # 确定性端到端自检后退出（Docker 判据）
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"task218-wafercausal/internal/httpapi"
	"task218-wafercausal/internal/service"
	"task218-wafercausal/internal/store"
)

func main() {
	dbPath := flag.String("db", "wafercausal.db", "SQLite database path")
	address := flag.String("addr", ":8080", "HTTP listen address")
	smoke := flag.Bool("smoke-test", false, "run deterministic end-to-end self-check and exit")
	flag.Parse()

	if *smoke {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// 自检必须从空库开始：使用临时数据库，保证每次运行确定性一致。
		tmpDB := *dbPath + ".smoke-" + fmt.Sprintf("%d", os.Getpid()) + ".db"
		stSmoke, err := store.Open(tmpDB)
		if err != nil {
			log.Fatalf("smoke open store: %v", err)
		}
		appSmoke := service.New(store.NewRepositories(stSmoke))
		if err := appSmoke.RunDemo(ctx); err != nil {
			stSmoke.Close()
			os.Remove(tmpDB)
			log.Fatalf("smoke test failed: %v", err)
		}
		// 关闭并重开同一数据库，验证持久化与重启恢复。
		if err := stSmoke.Close(); err != nil {
			log.Fatalf("smoke close store: %v", err)
		}
		stReopen, err := store.Open(tmpDB)
		if err != nil {
			log.Fatalf("smoke reopen store: %v", err)
		}
		appReopen := service.New(store.NewRepositories(stReopen))
		batches, err := appReopen.ListBatches(ctx)
		if err != nil || len(batches) == 0 {
			log.Fatalf("smoke restart check failed: batches=%d err=%v", len(batches), err)
		}
		defects, err := appReopen.ListDefects(ctx, batches[0].ID)
		if err != nil || len(defects) == 0 {
			log.Fatalf("smoke restart check failed: defects=%d err=%v", len(defects), err)
		}
		snaps, err := appReopen.ListSnapshots(ctx, batches[0].ID)
		if err != nil || len(snaps) == 0 {
			log.Fatalf("smoke restart check failed: snapshots=%d err=%v", len(snaps), err)
		}
		stats, err := appReopen.SelfCheck(ctx)
		if err != nil || stats["integrity_check"] != 1 {
			log.Fatalf("smoke selfcheck failed: %+v err=%v", stats, err)
		}
		stReopen.Close()
		os.Remove(tmpDB)
		fmt.Fprintln(os.Stdout, "task218-wafercausal smoke test passed")
		return
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	repos := store.NewRepositories(st)
	app := service.New(repos)

	srv := &http.Server{
		Addr:              *address,
		Handler:           httpapi.New(app).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("task218-wafercausal listening on %s (db=%s)", *address, *dbPath)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
