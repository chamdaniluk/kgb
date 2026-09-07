package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sicendikia/internal/store"
)

// fakeSIPPASN menyajikan respons /api/pegawai ala server asli.
func fakeSIPPASN(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pegawai" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pegawai": [
			{"status":"1","nip_pejabat":"199103312005011076","nama_pejabat":"GURU SYNC, S.Pd.","kd_jns_jab":"2","kd_jabatan":"30001","jabatan":"Guru Ahli Muda","kd_golongan":"32","golongan":"III/c","pangkat":"Penata","kd_unker":"03.08.46","unker":"SDN 3 Krangganharjo - Dinas Pendidikan","kd_esselon":"","esselon":""},
			{"status":"1","nip_pejabat":"199106242025212070","nama_pejabat":"PPPK SYNC, S.Pd","kd_jns_jab":"2","kd_jabatan":"30015","jabatan":"Guru Ahli Pertama","kd_golongan":"","golongan":"","pangkat":"","kd_unker":"03.11.22","unker":"SDN 3 Lemahputih - Dinas Pendidikan","kd_esselon":"","esselon":""},
			{"status":"1","nip_pejabat":"199001012025212099","nama_pejabat":"OPERATOR SYNC","kd_jns_jab":"3","kd_jabatan":"001","jabatan":"Operator Layanan Operasional","kd_golongan":"22","golongan":"II/c","pangkat":"Pengatur","kd_unker":"03","unker":"Dinas Pendidikan","kd_esselon":"","esselon":""}
		]}`))
	}))
}

func postSyncJSON(t *testing.T, client *http.Client, url, csrf string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp
}

func TestSyncPreviewTanpaMenulisDB(t *testing.T) {
	fx := newFixture(t)
	upstream := fakeSIPPASN(t)
	defer upstream.Close()
	t.Setenv("SIPPASN_BASE_URL", upstream.URL)

	adminClient, adminCSRF := loginClient(t, fx.srv.URL, "admin-dinas", "sandiadmin")
	resp := postSyncJSON(t, adminClient, fx.srv.URL+"/api/v1/admin/sync-sippasn/preview", adminCSRF, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview status=%d", resp.StatusCode)
	}
	var before int64
	if err := fx.pool.QueryRow(context.Background(), `SELECT count(*) FROM teachers`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if before != 2 {
		t.Fatalf("preview tidak boleh menulis: teachers=%d, want 2 (fixture)", before)
	}
}

func TestSyncEndToEndMembuatDanMemperbarui(t *testing.T) {
	fx := newFixture(t)
	upstream := fakeSIPPASN(t)
	defer upstream.Close()
	t.Setenv("SIPPASN_BASE_URL", upstream.URL)

	adminClient, adminCSRF := loginClient(t, fx.srv.URL, "admin-dinas", "sandiadmin")
	resp := postSyncJSON(t, adminClient, fx.srv.URL+"/api/v1/admin/sync-sippasn", adminCSRF, map[string]any{"limit": 0})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sync status=%d", resp.StatusCode)
	}
	var created, operator int
	if err := fx.pool.QueryRow(context.Background(),
		`SELECT count(*) FILTER (WHERE masa_kerja_source='sippasn'), count(*) FILTER (WHERE nip='199001012025212099') FROM teachers`).Scan(&created, &operator); err != nil {
		t.Fatal(err)
	}
	if created != 3 {
		t.Fatalf("baris dari SIPP ASN = %d, want 3 (2 guru + 1 operator)", created)
	}
	if operator != 1 {
		t.Fatalf("operator PPPK bergolongan harus tersinkron sebagai non_guru")
	}
	var kategori string
	if err := fx.pool.QueryRow(context.Background(),
		`SELECT kategori FROM teachers WHERE nip='199001012025212099'`).Scan(&kategori); err != nil {
		t.Fatal(err)
	}
	if kategori != "non_guru" {
		t.Fatalf("kategori operator = %q, want non_guru", kategori)
	}
	var golongan, asn string
	if err := fx.pool.QueryRow(context.Background(),
		`SELECT pangkat_gol, asn_type FROM teachers WHERE nip='199106242025212070'`).Scan(&golongan, &asn); err != nil {
		t.Fatal(err)
	}
	if golongan != "IX" || asn != "pppk" {
		t.Fatalf("PPPK baru = %s/%s, want IX/pppk", golongan, asn)
	}
	n, err := store.CountAudit(context.Background(), fx.pool, "sinkron_sippasn")
	if err != nil || n != 1 {
		t.Fatalf("audit sinkron_sippasn = %d (err=%v), want 1", n, err)
	}
	var asal string
	if err := fx.pool.QueryRow(context.Background(),
		`SELECT asal_data FROM bkn_imports WHERE file_name='sippasn:sync' ORDER BY id DESC LIMIT 1`).Scan(&asal); err != nil {
		t.Fatal(err)
	}
	if asal != "sippasn" {
		t.Fatalf("asal_data = %q, want sippasn", asal)
	}
	// Jalan kedua harus idempoten: update, bukan duplikat.
	resp2 := postSyncJSON(t, adminClient, fx.srv.URL+"/api/v1/admin/sync-sippasn", adminCSRF, map[string]any{"limit": 0})
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("sync kedua status=%d", resp2.StatusCode)
	}
	var total int64
	if err := fx.pool.QueryRow(context.Background(), `SELECT count(*) FROM teachers`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 5 {
		t.Fatalf("teachers setelah 2x sync = %d, want 5 (2 fixture + 3 sync)", total)
	}

	historyResp, err := adminClient.Get(fx.srv.URL + "/api/v1/admin/sync-sippasn/history?limit=5")
	if err != nil {
		t.Fatal(err)
	}
	defer historyResp.Body.Close()
	if historyResp.StatusCode != http.StatusOK {
		t.Fatalf("history status=%d", historyResp.StatusCode)
	}
	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(historyResp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) != 2 {
		t.Fatalf("riwayat = %d entri, want 2", len(envelope.Data))
	}
}

func TestSyncMenolakTanpaLogin(t *testing.T) {
	fx := newFixture(t)
	resp, err := http.Post(fx.srv.URL+"/api/v1/admin/sync-sippasn", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("tanpa sesi status=%d, want 401", resp.StatusCode)
	}
}

func TestSyncUpstreamMati(t *testing.T) {
	fx := newFixture(t)
	t.Setenv("SIPPASN_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("SIPPASN_TIMEOUT_SECONDS", "2")
	adminClient, adminCSRF := loginClient(t, fx.srv.URL, "admin-dinas", "sandiadmin")
	start := time.Now()
	resp := postSyncJSON(t, adminClient, fx.srv.URL+"/api/v1/admin/sync-sippasn", adminCSRF, nil)
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("upstream mati status=%d, want 502", resp.StatusCode)
	}
	if time.Since(start) > 30*time.Second {
		t.Fatal("timeout upstream terlalu lama")
	}
}
