// Package esign mengisolasi integrasi REST eSign Kominfo dari workflow aplikasi.
package esign

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client adalah HTTP client eSign Kominfo.
type Client struct {
	BaseURL  string
	Username string
	Password string
	HTTP     *http.Client
}

// SignRequest berisi data tanda tangan visible.
type SignRequest struct {
	NIK         string
	Passphrase  string
	ImageBase64 string
	Page        int
	OriginX     float64
	OriginY     float64
	Width       float64
	Height      float64
	PDF         []byte
}

// SignResponse adalah tanda terima dari eSign.
type SignResponse struct {
	ReceiptID string `json:"receipt_id"`
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (c *Client) endpoint(path string) (string, error) {
	if strings.TrimSpace(c.BaseURL) == "" {
		return "", errors.New("base URL eSign belum dikonfigurasi")
	}
	return strings.TrimRight(c.BaseURL, "/") + "/" + strings.TrimLeft(path, "/"), nil
}

// Sign mengirim PDF konsep ke endpoint v2 dengan mode VISIBLE.
func (c *Client) Sign(ctx context.Context, req SignRequest) (SignResponse, error) {
	if req.NIK == "" || req.Passphrase == "" || len(req.PDF) == 0 {
		return SignResponse{}, errors.New("NIK, passphrase, dan PDF wajib diisi")
	}
	url, err := c.endpoint("/api/v2/sign/pdf")
	if err != nil {
		return SignResponse{}, err
	}
	payload := map[string]any{
		"nik":        req.NIK,
		"passphrase": req.Passphrase,
		"file":       []string{base64.StdEncoding.EncodeToString(req.PDF)},
		"signatureProperties": []map[string]any{{
			"imageBase64": req.ImageBase64,
			"tampilan":    "VISIBLE",
			"page":        req.Page,
			"originX":     req.OriginX,
			"originY":     req.OriginY,
			"width":       req.Width,
			"height":      req.Height,
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return SignResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return SignResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.setBasicAuth(httpReq)
	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return SignResponse{}, fmt.Errorf("panggilan eSign: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return SignResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SignResponse{}, fmt.Errorf("eSign mengembalikan HTTP %d: %s", resp.StatusCode, sanitize(responseBody))
	}
	var decoded any
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return SignResponse{}, fmt.Errorf("respons eSign bukan JSON: %w", err)
	}
	receipt := findString(decoded, "id_dokumen", "idDokumen", "receipt_id", "document_id")
	if receipt == "" {
		return SignResponse{}, errors.New("respons eSign tidak memuat id_dokumen")
	}
	return SignResponse{ReceiptID: receipt}, nil
}

// Download mengambil PDF final berdasarkan tanda terima.
func (c *Client) Download(ctx context.Context, receiptID string) ([]byte, error) {
	if receiptID == "" {
		return nil, errors.New("id dokumen eSign kosong")
	}
	url, err := c.endpoint("/api/sign/download/" + url.PathEscape(receiptID))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.setBasicAuth(req)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("unduh hasil eSign: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unduh eSign HTTP %d: %s", resp.StatusCode, sanitize(body))
	}
	if len(body) < 5 || string(body[:5]) != "%PDF-" {
		return nil, errors.New("hasil eSign bukan PDF")
	}
	return body, nil
}

// Status mengecek kesiapan NIK tanpa mengembalikan rahasia.
func (c *Client) Status(ctx context.Context, nik string) error {
	if nik == "" {
		return errors.New("NIK kosong")
	}
	url, err := c.endpoint("/api/user/status/" + nik)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	c.setBasicAuth(req)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status eSign HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) setBasicAuth(req *http.Request) {
	if c.Username != "" || c.Password != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}
}

func sanitize(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 300 {
		return text[:300]
	}
	return text
}

func findString(v any, keys ...string) string {
	switch value := v.(type) {
	case map[string]any:
		for _, key := range keys {
			if s, ok := value[key].(string); ok && s != "" {
				return s
			}
			if n, ok := value[key].(float64); ok {
				return fmt.Sprintf("%.0f", n)
			}
		}
		for _, child := range value {
			if result := findString(child, keys...); result != "" {
				return result
			}
		}
	case []any:
		for _, child := range value {
			if result := findString(child, keys...); result != "" {
				return result
			}
		}
	}
	return ""
}
