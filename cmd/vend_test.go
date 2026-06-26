package cmd

import "testing"

func TestHealthEndpointNormalizesBaseURL(t *testing.T) {
	tests := map[string]string{
		"https://zapvend.com":      "https://zapvend.com/health",
		"https://zapvend.com/":     "https://zapvend.com/health",
		"https://zapvend.com/api":  "https://zapvend.com/health",
		"https://zapvend.com/api/": "https://zapvend.com/health",
	}

	for base, want := range tests {
		if got := healthEndpoint(base); got != want {
			t.Fatalf("healthEndpoint(%q) = %q, want %q", base, got, want)
		}
	}
}
