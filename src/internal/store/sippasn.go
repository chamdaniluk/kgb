package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultSIPPASNBaseURL = "https://sippasn.grobogan.go.id"

// SIPPASNOfficer adalah satu baris data pejabat dari endpoint publik SIPP ASN:
// GET {base}/api/pegawai -> {"pegawai":[{nip_pejabat, nama_pejabat,
// kd_jns_jab, kd_jabatan, jabatan, kd_golongan, golongan, pangkat,
// kd_unker, unker, kd_esselon, esselon, status}]}.
type SIPPASNOfficer struct {
	NIP        string
	Name       string
	JobKind    string
	JobCode    string
	JobTitle   string
	Golongan   string
	Pangkat    string
	UnitCode   string
	UnitName   string
	EselonCode string
	Eselon     string
	Status     string
}

// sippASNRow memetakan nama field wire persis seperti yang dikirim SIPP ASN.
type sippASNRow struct {
	Status     string `json:"status"`
	NIP        string `json:"nip_pejabat"`
	Name       string `json:"nama_pejabat"`
	JobKind    string `json:"kd_jns_jab"`
	JobCode    string `json:"kd_jabatan"`
	JobTitle   string `json:"jabatan"`
	GolCode    string `json:"kd_golongan"`
	Golongan   string `json:"golongan"`
	Pangkat    string `json:"pangkat"`
	UnitCode   string `json:"kd_unker"`
	UnitName   string `json:"unker"`
	EselonCode string `json:"kd_esselon"`
	Eselon     string `json:"esselon"`
}

// SIPPASNClient mengambil daftar pejabat dari SIPP ASN dengan timeout eksplisit.
// HTTP client disuntik agar test dapat memakai httptest tanpa jaringan.
type SIPPASNClient struct {
	BaseURL string
	HTTP    *http.Client
}

func (c SIPPASNClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (c SIPPASNClient) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return defaultSIPPASNBaseURL
}

// FetchOfficers mengunduh seluruh daftar pejabat. Respons SIPP ASN memakai
// JSON longgar (karakter kontrol di dalam string), jadi decoder non-strict.
func (c SIPPASNClient) FetchOfficers(ctx context.Context) ([]SIPPASNOfficer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL()+"/api/pegawai", nil)
	if err != nil {
		return nil, fmt.Errorf("buat request SIPP ASN: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SI-CENDIKIA/sync-asn")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("panggil SIPP ASN: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SIPP ASN status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("baca respons SIPP ASN: %w", err)
	}
	raw = bytes.ReplaceAll(raw, []byte("\r"), []byte(" "))
	raw = bytes.ReplaceAll(raw, []byte("\n"), []byte(" "))
	raw = bytes.ReplaceAll(raw, []byte("\t"), []byte(" "))
	var envelope struct {
		Officers []sippASNRow `json:"pegawai"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("urai respons SIPP ASN: %w", err)
	}
	result := make([]SIPPASNOfficer, 0, len(envelope.Officers))
	for _, row := range envelope.Officers {
		result = append(result, SIPPASNOfficer{
			NIP:        row.NIP,
			Name:       row.Name,
			JobKind:    row.JobKind,
			JobCode:    row.JobCode,
			JobTitle:   row.JobTitle,
			Golongan:   row.Golongan,
			Pangkat:    row.Pangkat,
			UnitCode:   row.UnitCode,
			UnitName:   row.UnitName,
			EselonCode: row.EselonCode,
			Eselon:     row.Eselon,
			Status:     row.Status,
		})
	}
	return result, nil
}
