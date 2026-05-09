package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/DankeyDevDave/pp-zapvend-cli/internal/config"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check all dependencies and configuration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		allOK := true
		check := func(name, detail string, ok bool) {
			status := "OK"
			if !ok {
				status = "FAIL"
				allOK = false
			}
			fmt.Printf("  [%-4s] %s", status, name)
			if detail != "" {
				fmt.Printf(": %s", detail)
			}
			fmt.Println()
		}

		fmt.Println("Checking dependencies...")

		// Python
		py, err := exec.LookPath("python3")
		if err != nil {
			py, err = exec.LookPath("python")
		}
		check("Python", py, err == nil)

		// Playwright
		playwrightOK := false
		playwrightDetail := "not installed (pip install playwright)"
		if err == nil {
			out, perr := exec.Command(py, "-c", "import playwright; print('ok')").Output()
			playwrightOK = perr == nil && strings.TrimSpace(string(out)) == "ok"
			if playwrightOK {
				playwrightDetail = ""
			}
		}
		check("Playwright", playwrightDetail, playwrightOK)

		// Chromium
		chromiumOK := false
		chromiumDetail := "not installed (playwright install chromium)"
		if playwrightOK {
			chromScript := `
from playwright.sync_api import sync_playwright
import os
with sync_playwright() as p:
    path = p.chromium.executable_path
    print("ok" if os.path.exists(path) else "missing")
`
			out, cerr := exec.Command(py, "-c", chromScript).Output()
			chromiumOK = cerr == nil && strings.TrimSpace(string(out)) == "ok"
			if chromiumOK {
				chromiumDetail = ""
			}
		}
		check("Chromium", chromiumDetail, chromiumOK)

		// config.yaml
		configPath, err := config.Detect()
		check("config.yaml", configPath, err == nil)

		if err == nil {
			// Automator script
			zapCfg, loadErr := config.Load(configPath)
			scriptOK := loadErr == nil
			if scriptOK {
				_, statErr := os.Stat(zapCfg.ScriptPath)
				scriptOK = statErr == nil
			}
			scriptDetail := ""
			if !scriptOK {
				scriptDetail = "not found"
			}
			check("Automator script", scriptDetail, scriptOK)

			// Accounts
			if loadErr == nil {
				accountsOK := len(zapCfg.Accounts) > 0
				detail := ""
				if !accountsOK {
					detail = "no cybervendit_accounts in config.yaml"
				} else {
					names := make([]string, 0, len(zapCfg.Accounts))
					for k := range zapCfg.Accounts {
						names = append(names, k)
					}
					detail = strings.Join(names, ", ")
				}
				check("CyberVendIT accounts", detail, accountsOK)

				// Streets
				streetsOK := len(zapCfg.Streets) > 0
				detail = ""
				if !streetsOK {
					detail = "no street sections found in config.yaml"
				} else {
					names := make([]string, 0, len(zapCfg.Streets))
					for k := range zapCfg.Streets {
						names = append(names, k)
					}
					detail = strings.Join(names, ", ")
				}
				check("Streets", detail, streetsOK)
			}
		}

		fmt.Println()
		if allOK {
			fmt.Println("All checks passed.")
		} else {
			fmt.Fprintln(os.Stderr, "Some checks failed — see above.")
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	// Doctor needs config access but should not fail if config is missing
	// Override PersistentPreRunE for this command
	doctorCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		cfg, _ = loadConfig() // ignore error — doctor reports it itself
		return nil
	}
	rootCmd.AddCommand(doctorCmd)
}
