package runner

import "testing"

func TestAPIEndpointNormalizesBaseURL(t *testing.T) {
	tests := map[string]string{
		"https://zapvend.com":      "https://zapvend.com/api/electricity/generate-token",
		"https://zapvend.com/":     "https://zapvend.com/api/electricity/generate-token",
		"https://zapvend.com/api":  "https://zapvend.com/api/electricity/generate-token",
		"https://zapvend.com/api/": "https://zapvend.com/api/electricity/generate-token",
	}

	for base, want := range tests {
		if got := apiEndpoint(base, "/electricity/generate-token"); got != want {
			t.Fatalf("apiEndpoint(%q) = %q, want %q", base, got, want)
		}
	}
}
