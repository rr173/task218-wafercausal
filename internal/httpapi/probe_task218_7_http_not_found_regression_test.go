package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"task218-wafercausal/internal/service"
	"task218-wafercausal/internal/store"
)

func TestTask218Bug07MissingBatchIsHTTPNotFound(t *testing.T) {
	st, err := store.Open("")
	if err != nil { t.Fatal(err) }
	defer st.Close()
	srv := httptest.NewServer(New(service.New(store.NewRepositories(st))).Handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/batches/999999")
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound { t.Fatalf("expected 404 for missing batch, got %d", resp.StatusCode) }
}
