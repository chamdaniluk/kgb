package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"sicendikia/internal/auth"
	"sicendikia/internal/store"
)

func seedAdminFilterFixture(t *testing.T, fx fixture) {
	t.Helper()
	ctx := context.Background()
	hash, err := auth.HashPassword("sandix")
	if err != nil {
		t.Fatal(err)
	}
	var korwil, sdn, smp, dinas int64
	if err := fx.pool.QueryRow(ctx, `INSERT INTO units (code, name, type) VALUES ('KORWILCAM-GROBOGAN','KORWILCAM GROBOGAN','korwil') RETURNING id`).Scan(&korwil); err != nil {
		t.Fatal(err)
	}
	if err := fx.pool.QueryRow(ctx, `INSERT INTO units (code, name, type, parent_id) VALUES ('SDN-1-X','SDN 1 Contoh','sd',$1) RETURNING id`, korwil).Scan(&sdn); err != nil {
		t.Fatal(err)
	}
	if err := fx.pool.QueryRow(ctx, `INSERT INTO units (code, name, type) VALUES ('SMPN-1-X','SMP NEGERI 1 Contoh','smp') RETURNING id`).Scan(&smp); err != nil {
		t.Fatal(err)
	}
	if err := fx.pool.QueryRow(ctx, `INSERT INTO units (code, name, type) VALUES ('DINAS-PENDIDIKAN','DINAS PENDIDIKAN','dinas') RETURNING id`).Scan(&dinas); err != nil {
		t.Fatal(err)
	}
	seedUser := func(username, role, name string, unit any) {
		t.Helper()
		if _, err := fx.pool.Exec(ctx, `INSERT INTO users (username, password_hash, role, name, unit_id) VALUES ($1,$2,$3,$4,$5)`, username, hash, role, name, unit); err != nil {
			t.Fatal(err)
		}
	}
	seedUser("petugas.unit1", "verifikator_unit", "Petugas Unit SD", sdn)
	seedUser("petugas.smp1", "verifikator_unit", "Petugas Unit SMP", smp)
	seedUser("verif.dinas1", "verifikator_dinas", "Verifikator Dinas", dinas)
	seedUser("pimpinan1", "pimpinan", "Kepala Dinas", dinas)
	if _, err := fx.pool.Exec(ctx, `INSERT INTO teachers (nip, name, asn_type, kategori, unit_id, pangkat_gol, masa_kerja_tahun) VALUES
		('111111111111111111','Guru SD Contoh','pns','guru',$1,'III/a',4),
		('222222222222222222','Guru SMP Contoh','pns','guru',$2,'III/b',6)`, sdn, smp); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.pool.Exec(ctx, `INSERT INTO audit_logs (actor_user_id, action, ip) VALUES
		((SELECT id FROM users WHERE username='admin-dinas'),'login','127.0.0.1'),
		((SELECT id FROM users WHERE username='petugas.unit1'),'setuju_unit','127.0.0.1')`); err != nil {
		t.Fatal(err)
	}
}

func getAdminJSON(t *testing.T, client *http.Client, path string) (int, map[string]any) {
	t.Helper()
	resp, err := client.Get(path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func dataList(t *testing.T, out map[string]any) []any {
	t.Helper()
	data, ok := out["data"].([]any)
	if !ok {
		t.Fatalf("respons tanpa data list: %v", out)
	}
	return data
}

func TestAdminUsersRoleGroupTabs(t *testing.T) {
	fx := newFixture(t)
	seedAdminFilterFixture(t, fx)
	client, _ := loginClient(t, fx.srv.URL, "admin-dinas", "sandiadmin")
	expect := map[string]int{"admin": 1, "guru": 2, "unit": 2, "dinas": 2}
	for group, want := range expect {
		code, out := getAdminJSON(t, client, fx.srv.URL+"/api/v1/admin/users?role_group="+group+"&limit=0&offset=0")
		if code != http.StatusOK {
			t.Fatalf("group %s status=%d", group, code)
		}
		if got := len(dataList(t, out)); got != want {
			t.Errorf("group %s = %d akun, want %d", group, got, want)
		}
		for _, item := range dataList(t, out) {
			m := item.(map[string]any)
			if m["role_group"] != group {
				t.Errorf("group %s memuat role_group=%v", group, m["role_group"])
			}
		}
	}
}

func TestAdminUsersRejectsBadRoleGroup(t *testing.T) {
	fx := newFixture(t)
	client, _ := loginClient(t, fx.srv.URL, "admin-dinas", "sandiadmin")
	if code, _ := getAdminJSON(t, client, fx.srv.URL+"/api/v1/admin/users?role_group=bupati"); code != http.StatusBadRequest {
		t.Fatalf("role_group asing status=%d, want 400", code)
	}
}

func TestAdminUsersUnitFilters(t *testing.T) {
	fx := newFixture(t)
	seedAdminFilterFixture(t, fx)
	client, _ := loginClient(t, fx.srv.URL, "admin-dinas", "sandiadmin")
	code, out := getAdminJSON(t, client, fx.srv.URL+"/api/v1/admin/users?role_group=unit&kecamatan=GROBOGAN&limit=0&offset=0")
	if code != http.StatusOK {
		t.Fatalf("status=%d", code)
	}
	items := dataList(t, out)
	if len(items) != 1 || items[0].(map[string]any)["username"] != "petugas.unit1" {
		t.Errorf("filter unit+GROBOGAN salah: %v", out)
	}
	code, out = getAdminJSON(t, client, fx.srv.URL+"/api/v1/admin/users?role_group=dinas&unit_type=dinas&limit=0&offset=0")
	if code != http.StatusOK {
		t.Fatalf("status=%d", code)
	}
	if len(dataList(t, out)) != 2 {
		t.Errorf("filter dinas salah: %v", out)
	}
}

func TestAdminTeachersKecamatanFilter(t *testing.T) {
	fx := newFixture(t)
	seedAdminFilterFixture(t, fx)
	client, _ := loginClient(t, fx.srv.URL, "admin-dinas", "sandiadmin")
	code, out := getAdminJSON(t, client, fx.srv.URL+"/api/v1/admin/teachers?kecamatan=GROBOGAN&limit=0&offset=0")
	if code != http.StatusOK {
		t.Fatalf("status=%d", code)
	}
	items := dataList(t, out)
	if len(items) != 1 {
		t.Fatalf("filter GROBOGAN = %d guru, want 1 (SD di bawah korwil)", len(items))
	}
	if items[0].(map[string]any)["nip"] != "111111111111111111" {
		t.Errorf("guru yang cocok salah: %v", items[0])
	}
	code, out = getAdminJSON(t, client, fx.srv.URL+"/api/v1/admin/teachers?unit_type=smp&limit=0&offset=0")
	if code != http.StatusOK {
		t.Fatalf("status=%d", code)
	}
	items = dataList(t, out)
	if len(items) != 1 || items[0].(map[string]any)["nip"] != "222222222222222222" {
		t.Errorf("filter smp salah: %v", out)
	}
}

func TestAdminAuditActorAndDateFilter(t *testing.T) {
	fx := newFixture(t)
	seedAdminFilterFixture(t, fx)
	client, _ := loginClient(t, fx.srv.URL, "admin-dinas", "sandiadmin")
	code, out := getAdminJSON(t, client, fx.srv.URL+"/api/v1/admin/audit-logs?actor=petugas.unit1")
	if code != http.StatusOK {
		t.Fatalf("status=%d", code)
	}
	items := dataList(t, out)
	if len(items) != 1 || items[0].(map[string]any)["action"] != "setuju_unit" {
		t.Errorf("filter actor salah: %v", out)
	}
	code, out = getAdminJSON(t, client, fx.srv.URL+"/api/v1/admin/audit-logs?action=login&from=2000-01-01&to=2100-01-01")
	if code != http.StatusOK {
		t.Fatalf("status=%d", code)
	}
	if len(dataList(t, out)) < 1 {
		t.Errorf("rentang tanggal luas harus memuat login: %v", out)
	}
	code, _ = getAdminJSON(t, client, fx.srv.URL+"/api/v1/admin/audit-logs?from=besok")
	if code != http.StatusBadRequest {
		t.Fatalf("tanggal salah status=%d, want 400", code)
	}
}

func TestAdminFilterOptionsAndActions(t *testing.T) {
	fx := newFixture(t)
	seedAdminFilterFixture(t, fx)
	client, _ := loginClient(t, fx.srv.URL, "admin-dinas", "sandiadmin")
	code, out := getAdminJSON(t, client, fx.srv.URL+"/api/v1/admin/filter-options")
	if code != http.StatusOK {
		t.Fatalf("filter-options status=%d", code)
	}
	data := out["data"].(map[string]any)
	if len(data["units"].([]any)) < 4 {
		t.Errorf("units kurang: %v", data)
	}
	found := false
	for _, k := range data["kecamatan"].([]any) {
		if k == "GROBOGAN" {
			found = true
		}
	}
	if !found {
		t.Errorf("kecamatan tanpa GROBOGAN: %v", data["kecamatan"])
	}
	code, out = getAdminJSON(t, client, fx.srv.URL+"/api/v1/admin/audit-logs/actions")
	if code != http.StatusOK {
		t.Fatalf("actions status=%d", code)
	}
	joined := strings.Join(toStrings(t, out["data"]), ",")
	if !strings.Contains(joined, "login") || !strings.Contains(joined, "setuju_unit") {
		t.Errorf("daftar aksi tidak lengkap: %s", joined)
	}
}

func toStrings(t *testing.T, v any) []string {
	t.Helper()
	items, ok := v.([]any)
	if !ok {
		t.Fatalf("bukan list: %v", v)
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		s, _ := it.(string)
		out = append(out, s)
	}
	return out
}

var _ = store.RoleGroupAdmin
