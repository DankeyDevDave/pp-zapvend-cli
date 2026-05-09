package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var nonStreetKeys = map[string]bool{
	"cybervendit_accounts":  true,
	"meter_account_mapping": true,
	"cybervendit":           true,
	"database":              true,
}

type AccountConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	BaseURL  string `yaml:"base_url"`
}

type TariffConfig struct {
	FlatRate   float64 `yaml:"flat_rate"`
	Tier1Rate  float64 `yaml:"tier1_rate"`
	Tier2Rate  float64 `yaml:"tier2_rate"`
	Tier1Limit float64 `yaml:"tier1_limit"`
	VATRate    float64 `yaml:"vat_rate"`
}

type UserEntry struct {
	Name            string  `yaml:"name"`
	MonthlyPurchase float64 `yaml:"monthly_purchase"`
	MeterType       string  `yaml:"meter_type"`
}

type StreetConfig struct {
	Tariffs    TariffConfig         `yaml:"tariffs"`
	VendingFee float64              `yaml:"vending_fee"`
	Users      map[string]UserEntry `yaml:"users"`
	Name       string               `yaml:"name"`
	Paths      struct {
		DataFile   string `yaml:"data_file"`
		ReceiptDir string `yaml:"receipt_dir"`
	} `yaml:"paths"`
}

type Config struct {
	Accounts            map[string]AccountConfig
	MeterAccountMapping map[string]string
	Streets             map[string]StreetConfig
	ZapvendDir          string
	ScriptPath          string
}

type MeterInfo struct {
	MeterNumber string
	StreetKey   string
	Street      StreetConfig
	User        UserEntry
}

func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	zapvendDir := filepath.Dir(configPath)
	cfg := &Config{
		Accounts:            make(map[string]AccountConfig),
		MeterAccountMapping: make(map[string]string),
		Streets:             make(map[string]StreetConfig),
		ZapvendDir:          zapvendDir,
		ScriptPath:          filepath.Join(zapvendDir, "cybervendit_automator_with_logging.py"),
	}

	for key, node := range raw {
		switch {
		case key == "cybervendit_accounts":
			if err := node.Decode(&cfg.Accounts); err != nil {
				return nil, fmt.Errorf("decode accounts: %w", err)
			}
		case key == "meter_account_mapping":
			if err := node.Decode(&cfg.MeterAccountMapping); err != nil {
				return nil, fmt.Errorf("decode meter mapping: %w", err)
			}
		case nonStreetKeys[key]:
			// skip
		default:
			var sc StreetConfig
			if err := node.Decode(&sc); err == nil && (sc.Name != "" || len(sc.Users) > 0) {
				cfg.Streets[key] = sc
			}
		}
	}

	return cfg, nil
}

// FindMeter looks up a meter by name (partial, case-insensitive) or meter number.
func (c *Config) FindMeter(query string) (*MeterInfo, error) {
	query = strings.TrimSpace(query)
	queryLower := strings.ToLower(query)

	// Exact meter number match first
	for key, street := range c.Streets {
		if user, ok := street.Users[query]; ok {
			return &MeterInfo{
				MeterNumber: query,
				StreetKey:   key,
				Street:      street,
				User:        user,
			}, nil
		}
	}

	// Name search
	for key, street := range c.Streets {
		for meterNum, user := range street.Users {
			if strings.Contains(strings.ToLower(user.Name), queryLower) {
				return &MeterInfo{
					MeterNumber: meterNum,
					StreetKey:   key,
					Street:      street,
					User:        user,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no meter found matching %q", query)
}

// AllMeters returns all meters across all streets.
func (c *Config) AllMeters() []MeterInfo {
	var result []MeterInfo
	for key, street := range c.Streets {
		for meterNum, user := range street.Users {
			result = append(result, MeterInfo{
				MeterNumber: meterNum,
				StreetKey:   key,
				Street:      street,
				User:        user,
			})
		}
	}
	return result
}

// CurrentMonthlyKWh reads the users_data JSON for a street and returns the
// current calendar month's kWh total for the given meter.
func (c *Config) CurrentMonthlyKWh(street StreetConfig, meterNumber string) float64 {
	if street.Paths.DataFile == "" {
		return 0
	}
	dataPath := filepath.Join(c.ZapvendDir, street.Paths.DataFile)
	data, err := os.ReadFile(dataPath)
	if err != nil {
		return 0
	}

	var users []struct {
		MeterNumber string `json:"meter_number"`
		Vends       []struct {
			Date string  `json:"date"`
			KWh  float64 `json:"kwh"`
		} `json:"vends"`
	}
	if err := json.Unmarshal(data, &users); err != nil {
		return 0
	}

	now := time.Now()
	for _, u := range users {
		if u.MeterNumber != meterNumber {
			continue
		}
		var total float64
		for _, v := range u.Vends {
			t := parseDate(v.Date)
			if t.IsZero() {
				continue
			}
			if t.Year() == now.Year() && t.Month() == now.Month() {
				total += v.KWh
			}
		}
		return total
	}
	return 0
}

func parseDate(s string) time.Time {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// APIURL returns the Zapvend API base URL.
// Defaults to https://zapvend.com if ZAPVEND_API_URL is not set.
func APIURL() string {
	if u := os.Getenv("ZAPVEND_API_URL"); u != "" {
		return u
	}
	return "https://zapvend.com"
}

// CLISecret returns the shared CLI secret from ZAPVEND_CLI_SECRET env var.
func CLISecret() string {
	return os.Getenv("ZAPVEND_CLI_SECRET")
}

// Detect tries to find config.yaml from the binary location, then falls back
// to common known paths. ZAPVEND_CONFIG env var takes priority.
func Detect() (string, error) {
	if p := os.Getenv("ZAPVEND_CONFIG"); p != "" {
		return p, nil
	}

	// Resolve binary → follow symlinks → look for ../config.yaml
	exe, err := os.Executable()
	if err == nil {
		resolved, err := filepath.EvalSymlinks(exe)
		if err == nil {
			candidate := filepath.Join(filepath.Dir(resolved), "..", "config.yaml")
			if _, err := os.Stat(candidate); err == nil {
				return filepath.Clean(candidate), nil
			}
		}
	}

	// Known fallback
	home, _ := os.UserHomeDir()
	fallback := filepath.Join(home, "DevFolder", "Zapvend", "config.yaml")
	if _, err := os.Stat(fallback); err == nil {
		return fallback, nil
	}

	return "", fmt.Errorf("config.yaml not found; set ZAPVEND_CONFIG to its path")
}
