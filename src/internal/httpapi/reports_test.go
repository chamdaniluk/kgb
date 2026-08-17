package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"sicendikia/internal/auth"
)

func TestIssuedHistoryHanyaScopeUnit(t *testing.T) {
	fx := newFixture(t)
	ctx := context.Background()
	hash := func(pw string) string {
		v, err := auth.HashPassword(pw)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	var unitID, otherUnit int64
	if err := fx.pool.QueryRow(ctx, `SELECT id FROM units WHERE code='KORWIL-01'`).Scan(&unitID); err != nil {
		t.Fatal(err)
	}
	if err := fx.pool.QueryRow(ctx, `INSERT INTO units (code,name,type) VALUES ('SMP-LAIN','SMP Lain','smp') RETURNING id`).Scan(&otherUnit); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.pool.Exec(ctx, `INSERT INTO users (username,password_hash,role,name,unit_id) VALUES ('unit-scope',$1,'verifikator_unit','Unit Scope',$2)`, hash("sandi"), unitID); err != nil {
		t.Fatal(err)
	}
	var teacherID, otherTeacher, otherUser, tpl, signer int64
	if err := fx.pool.QueryRow(ctx, `SELECT id FROM teachers WHERE nip=$1`, nipPNS).Scan(&teacherID); err != nil {
		t.Fatal(err)
	}
	if err := fx.pool.QueryRow(ctx, `INSERT INTO users (username,password_hash,role,name,unit_id) VALUES ('guru-lain',$1,'asn','Guru Lain',$2) RETURNING id`, hash("x"), otherUnit).Scan(&otherUser); err != nil {
		t.Fatal(err)
	}
	if err := fx.pool.QueryRow(ctx, `INSERT INTO teachers (user_id,nip,name,asn_type,unit_id,pangkat_gol,masa_kerja_tahun) VALUES ($1,'198801012006011099','Guru Lain','pns',$2,'III/b',8) RETURNING id`, otherUser, otherUnit).Scan(&otherTeacher); err != nil {
		t.Fatal(err)
	}
	if err := fx.pool.QueryRow(ctx, `SELECT id FROM letter_number_templates WHERE is_active LIMIT 1`).Scan(&tpl); err != nil {
		t.Fatal(err)
	}
	if err := fx.pool.QueryRow(ctx, `INSERT INTO users (username,password_hash,role,name) VALUES ('signer-hist',$1,'pimpinan','Signer') RETURNING id`, hash("s")).Scan(&signer); err != nil {
		t.Fatal(err)
	}
	var sid1, sid2 int64
	if err := fx.pool.QueryRow(ctx, `INSERT INTO submissions (teacher_id,status,proposed_tmt,current_salary,next_salary,snapshot_name,snapshot_nip,snapshot_unit_id,snapshot_unit_name) VALUES ($1,'terbit','2026-04-01',1,2,'Guru PNS Contoh',$2,$3,'Korwil') RETURNING id`, teacherID, nipPNS, unitID).Scan(&sid1); err != nil {
		t.Fatal(err)
	}
	if err := fx.pool.QueryRow(ctx, `INSERT INTO submissions (teacher_id,status,proposed_tmt,current_salary,next_salary,snapshot_name,snapshot_nip,snapshot_unit_id,snapshot_unit_name) VALUES ($1,'terbit','2026-04-01',1,2,'Guru Lain','198801012006011099',$2,'SMP Lain') RETURNING id`, otherTeacher, otherUnit).Scan(&sid2); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.pool.Exec(ctx, `INSERT INTO letters (submission_id,template_id,number,pdf_path,signer_user_id) VALUES ($1,$2,'800/101/4.2/2026','/tmp/a.pdf',$3)`, sid1, tpl, signer); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.pool.Exec(ctx, `INSERT INTO letters (submission_id,template_id,number,pdf_path,signer_user_id) VALUES ($1,$2,'800/102/4.2/2026','/tmp/b.pdf',$3)`, sid2, tpl, signer); err != nil {
		t.Fatal(err)
	}
	client, _ := loginClient(t, fx.srv.URL, "unit-scope", "sandi")
	resp, err := client.Get(fx.srv.URL + "/api/v1/reports/issued")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var env struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 {
		t.Fatalf("riwayat unit = %d, ingin 1 (hanya scope)", len(env.Data))
	}
}

func TestNominasiMemakaiJendelaEnamSampaiTigaBulan(t *testing.T) {
	fx := newFixture(t)
	ctx := context.Background()
	next := time.Now().AddDate(0, 4, 0)
	if _, err := fx.pool.Exec(ctx, `UPDATE teachers SET tmt_kgb_last=$1 WHERE nip=$2`, next.AddDate(-2, 0, 0), nipPNS); err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("sandi")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.pool.Exec(ctx, `INSERT INTO users (username,password_hash,role,name) VALUES ('dinas-nom',$1,'admin_dinas','Dinas Nom')`, hash); err != nil {
		t.Fatal(err)
	}
	client, _ := loginClient(t, fx.srv.URL, "dinas-nom", "sandi")
	resp, err := client.Get(fx.srv.URL + "/api/v1/reports/nominations?bucket=nominasi")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var env struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range env.Data {
		if item["nip"] == nipPNS {
			found = true
			if item["bucket"] != "nominasi" {
				t.Fatalf("bucket = %v, ingin nominasi", item["bucket"])
			}
		}
	}
	if !found {
		t.Fatal("guru PNS tidak muncul di nominasi")
	}
}
