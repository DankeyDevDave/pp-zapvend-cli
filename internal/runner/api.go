package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type APIVendRequest struct {
	MeterNumber string  `json:"meter_number"`
	Amount      float64 `json:"amount"`
	DemoMode    bool    `json:"demo_mode"`
	FastMode    bool    `json:"fast_mode"`
}

type APIVendResponse struct {
	Message     string  `json:"message"`
	MeterNumber string  `json:"meter_number"`
	Amount      float64 `json:"amount"`
	TotalKWh    float64 `json:"total_kwh"`
	Token       string  `json:"token"`
	Timestamp   string  `json:"timestamp"`
	// error path
	Detail any `json:"detail,omitempty"`
}

// GenerateTokenViaAPI calls the Zapvend API's generate-token endpoint. Returns
// a GenerateTokenResult on success, or an error if the API is unreachable or
// returns a failure. A connection-refused error is returned unwrapped so the
// caller can distinguish "API down" from "API returned an error".
func GenerateTokenViaAPI(baseURL string, cliSecret string, meter string, rands float64, demoMode bool, fastMode bool) (*GenerateTokenResult, error) {
	url := apiEndpoint(baseURL, "/electricity/generate-token")

	body, _ := json.Marshal(APIVendRequest{
		MeterNumber: meter,
		Amount:      rands,
		DemoMode:    demoMode,
		FastMode:    fastMode,
	})

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cliSecret != "" {
		req.Header.Set("X-CLI-Secret", cliSecret)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err // connection-level error — API likely not running
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Detail any `json:"detail"`
		}
		_ = json.Unmarshal(raw, &errResp)
		return nil, fmt.Errorf("API error %d: %v", resp.StatusCode, errResp.Detail)
	}

	var apiResp APIVendResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return nil, fmt.Errorf("parse API response: %w", err)
	}

	isDemo := demoMode
	return &GenerateTokenResult{
		Success: true,
		Token:   apiResp.Token,
		Amount:  apiResp.TotalKWh,
		Meter:   meter,
		Demo:    isDemo,
	}, nil
}

func apiEndpoint(baseURL string, path string) string {
	base := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(base, "/api") {
		return base + path
	}
	return base + "/api" + path
}
