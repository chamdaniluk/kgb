package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"sicendikia/internal/auth"
	"sicendikia/internal/store"
)

// Admin Dinas: boleh approve usulan pegawai Dinas, ditolak untuk jenjang lain.
func TestAdminDinasHanyaProsesUsulanDinas(t *testing.T) {
	fx := newFixture(t)
	ctx := context.Background()
	hash := func(pw string) string {
		h, err := auth.HashPassword(pw)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	var korwil int64
	if err := fx.pool.QueryRow(ctx, `INSERT INTO units (code,name,type,district) VALUES ('KW-BRT','KORWILCAM BRATI','korwil','BRATI') RETURNING id`).Scan(&korwil); err != nil {
		t.Fatal(err)
	}
	var sd int64
	if err := fx.pool.QueryRow(ctx, `INSERT INTO units (code,name,type,district) VALUES ('SD-BRT','SDN 1 BRATI','sd','BRATI') RETURNING id`).Scan(&sd); err != nil {
		t.Fatal(err)
	}
	var seksi int64
	if err := fx.pool.QueryRow(ctx, `INSERT INTO units (code,name,type,district) VALUES ('SKS-PSDM','Seksi PSDM','dinas','DINAS') RETURNING id`).Scan(&seksi); err != nil {
		t.Fatal(err)
	}
	mkASN := func(nip, name string, unit int64) {
		var uid int64
		if err := fx.pool.QueryRow(ctx, `INSERT INTO users (username,password_hash,role,name,unit_id) VALUES ($1,$2,'asn',$3,$4) RETURNING id`, nip, hash(nip), name, unit).Scan(&uid); err != nil {
			t.Fatal(err)
		}
		if _, err := fx.pool.Exec(ctx, `INSERT INTO teachers (user_id,nip,name,asn_type,unit_id,pangkat_gol,masa_kerja_tahun,masa_kerja_source,tmt_awal) VALUES ($1,$2,$3,'pns',$4,'III/b',10,'tmt_cpns','2016-04-01')`, uid, nip, name, unit); err != nil {
			t.Fatal(err)
		}
	}
	mkASN("198001012005011011", "Guru SD Brati", sd)
	mkASN("198001012005011012", "Pegawai PSDM", seksi)
	if _, err := fx.pool.Exec(ctx, `INSERT INTO users (username,password_hash,role,name) VALUES ('adm-dinas',$1,'admin_dinas','Admin Dinas')`, hash("sandi")); err != nil {
		t.Fatal(err)
	}

	submit := func(nip string) map[string]any {
		client, csrf := loginClient(t, fx.srv.URL, nip, nip)
		// Guru III/b; KGB terakhir III/a; KP III/b (pola kp_kgb_test).
		resp, data := submitPDFWith(t, client, fx.srv.URL, csrf, map[string]string{
			"pangkat_gol": "III/a", "last_kgb_golongan": "III/a",
			"last_kp_golongan": "III/b", "last_kp_tmt": "2025-07-01",
			"last_kp_masa_tahun": "5", "last_kp_masa_bulan": "0",
			"last_kp_nomor": "800.1.3.2/795/2025", "last_kp_tanggal": "2025-06-20",
			"last_kp_pejabat": "BUPATI GROBOGAN",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("submit %s: status=%d body=%v", nip, resp.StatusCode, data)
		}
		return data
	}
	subSD := submit("198001012005011011")
	subDinas := submit("198001012005011012")
	if subSD["status"] != "menunggu_unit" {
		t.Fatalf("status SD = %v, ingin menunggu_unit", subSD["status"])
	}
	if subDinas["status"] != "menunggu_dinas" {
		t.Fatalf("status dinas = %v, ingin menunggu_dinas", subDinas["status"])
	}
	idOf := func(v any) int64 {
		f, ok := v.(map[string]any)["id"].(float64)
		if !ok {
			t.Fatalf("id usulan tidak terbaca: %v", v)
		}
		return int64(f)
	}
	// Tahap unit disetujui langsung via store agar fokus ke batasan dinas.
	var verifUnit int64
	if err := fx.pool.QueryRow(ctx, `INSERT INTO users (username,password_hash,role,name,unit_id) VALUES ('kwc-brati',$1,'verifikator_unit','Korwil Brati',$2) RETURNING id`, hash("sandi"), korwil).Scan(&verifUnit); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Transition(ctx, fx.pool, idOf(subSD), verifUnit, "menunggu_unit", "menunggu_dinas", "setuju_unit", "", "127.0.0.1"); err != nil {
		t.Fatalf("setuju unit: %v", err)
	}

	approve := func(user string, id int64, want int) map[string]any {
		client, csrf := loginClient(t, fx.srv.URL, user, "sandi")
		resp := postJSON(t, client, fx.srv.URL+"/api/v1/verifications/dinas/"+fmt.Sprint(id)+"/approve", csrf, `{"note":""}`)
		defer resp.Body.Close()
		out := decodeBody(t, resp)
		if resp.StatusCode != want {
			t.Fatalf("approve %s id=%d: status=%d ingin %d body=%v", user, id, resp.StatusCode, want, out)
		}
		return out
	}
	// Admin dinas ditolak untuk jenjang SD.
	approve("adm-dinas", idOf(subSD), http.StatusForbidden)
	// Admin dinas boleh untuk usulan dinas.
	got := approve("adm-dinas", idOf(subDinas), http.StatusOK)
	if d, ok := got["data"].(map[string]any); !ok || d["status"] != "menunggu_tte" {
		t.Fatalf("approve dinas = %v, ingin menunggu_tte", got)
	}
}
