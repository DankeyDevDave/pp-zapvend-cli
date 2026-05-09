package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type GenerateTokenResult struct {
	Success bool    `json:"success"`
	Token   string  `json:"token,omitempty"`
	Amount  float64 `json:"amount,omitempty"`
	Meter   string  `json:"meter,omitempty"`
	Demo    bool    `json:"demo,omitempty"`
	Error   string  `json:"error,omitempty"`
}

type CheckBalanceResult struct {
	Success         bool   `json:"success"`
	RemainingTokens int    `json:"remaining_tokens,omitempty"`
	Error           string `json:"error,omitempty"`
}

type Options struct {
	ScriptPath string
	Street     string
	DemoMode   bool
	ForceReal  bool
	Fast       bool
	Visible    bool
	LogLevel   string
}

func python() string {
	for _, name := range []string{"python3", "python"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return "python3"
}

func GenerateToken(meter string, kwh float64, opts Options) (*GenerateTokenResult, error) {
	args := []string{
		opts.ScriptPath,
		"--meter", meter,
		"--amount", fmt.Sprintf("%.2f", kwh),
		"--street", opts.Street,
		"--log-level", logLevel(opts.LogLevel),
		"--no-file-logs",
	}
	if opts.DemoMode {
		args = append(args, "--demo")
	}
	if opts.ForceReal {
		args = append(args, "--force-real")
	}
	if opts.Fast {
		args = append(args, "--fast")
	}
	if opts.Visible {
		args = append(args, "--visible")
	}

	out, err := runScript(opts.ScriptPath, args)
	if err != nil {
		return &GenerateTokenResult{Success: false, Error: err.Error()}, err
	}

	line := extractResultLine(out)
	var result GenerateTokenResult
	if err := json.Unmarshal([]byte(line), &result); err != nil {
		return nil, fmt.Errorf("parse output: %w (raw: %s)", err, truncate(out, 200))
	}
	return &result, nil
}

func CheckBalance(street string, opts Options) (*CheckBalanceResult, error) {
	args := []string{
		opts.ScriptPath,
		"--meter", "00000000000",
		"--amount", "1",
		"--street", street,
		"--balance-only",
		"--log-level", logLevel(opts.LogLevel),
		"--no-file-logs",
	}
	if opts.Visible {
		args = append(args, "--visible")
	}

	out, err := runScript(opts.ScriptPath, args)
	if err != nil {
		return &CheckBalanceResult{Success: false, Error: err.Error()}, err
	}

	line := extractResultLine(out)
	var result CheckBalanceResult
	if err := json.Unmarshal([]byte(line), &result); err != nil {
		return nil, fmt.Errorf("parse output: %w (raw: %s)", err, truncate(out, 200))
	}
	return &result, nil
}

func runScript(scriptPath string, args []string) (string, error) {
	if _, err := os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("automator script not found at %s: %w", scriptPath, err)
	}

	py := python()
	cmd := exec.Command(py, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if msg := extractLastErrorMessage(stderr.String()); msg != "" {
			return "", fmt.Errorf("%s", msg)
		}
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("%s", errMsg)
	}

	return stdout.String(), nil
}

// extractLastErrorMessage extracts the human-readable message from the last
// ERROR-level log line. Handles both JSON structured logs and the text format
// emitted by enhanced_logging_config: "DATE - ERROR - [module:func:line] - message".
func extractLastErrorMessage(raw string) string {
	var last string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// JSON format: {"level": "ERROR", "message": "..."}
		if line[0] == '{' {
			var entry struct {
				Level   string `json:"level"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal([]byte(line), &entry); err == nil &&
				strings.ToUpper(entry.Level) == "ERROR" && entry.Message != "" {
				last = entry.Message
			}
			continue
		}
		// Text format: "DATE - ERROR - [module:func:line] - message"
		// Strip ANSI colour codes before parsing.
		plain := stripANSI(line)
		parts := strings.SplitN(plain, " - ", 4)
		if len(parts) == 4 && strings.TrimSpace(parts[1]) == "ERROR" {
			last = strings.TrimSpace(parts[3])
		}
	}
	return last
}

// stripANSI removes ANSI escape sequences from s.
func stripANSI(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++ // skip 'm'
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func extractResultLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Result: ") {
			return strings.TrimPrefix(line, "Result: ")
		}
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return strings.TrimSpace(lines[i])
		}
	}
	return output
}

func logLevel(l string) string {
	if l == "" {
		return "INFO"
	}
	return strings.ToUpper(l)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
