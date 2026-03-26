package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type helperConfig struct {
	Runner string `json:"runner"`
}

func runDoctorCommand(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "tooie doctor: %v\n", err)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "tooie doctor: unexpected arguments")
		return 2
	}
	env := detectSetupEnv()
	fmt.Println("Tooie Doctor")
	fmt.Printf("  platform: %s\n", runtime.GOOS)
	fmt.Printf("  termux:   %s\n", onOffFlag(env.IsTermux))
	fmt.Printf("  tmux:     %s\n", yesNo(commandExists("tmux")))
	fmt.Printf("  jq:       %s\n", yesNo(commandExists("jq")))
	fmt.Printf("  matugen:  %s\n", yesNo(commandExists("matugen")))
	fmt.Printf("  fish:     %s\n", yesNo(commandExists("fish")))
	fmt.Printf("  starship: %s\n", yesNo(commandExists("starship")))
	fmt.Printf("  peaclock: %s\n", yesNo(commandExists("peaclock")))
	fmt.Printf("  rish:     %s\n", yesNo(commandExists("rish")))
	fmt.Printf("  tsu:      %s\n", yesNo(commandExists("tsu")))
	fmt.Printf("  su:       %s\n", yesNo(commandExists("su")))

	fmt.Println()
	fmt.Println("Dependency guidance (no automatic install):")
	if env.IsTermux {
		if commandExists("pacman") {
			fmt.Println("  pacman -S --needed --noconfirm tmux jq fish starship peaclock matugen")
		} else {
			fmt.Println("  pkg install -y tmux jq fish starship peaclock matugen")
		}
	} else {
		fmt.Println("  Install with your distro package manager: tmux jq fish starship peaclock matugen")
		fmt.Println("  Example (Debian/Ubuntu): sudo apt install tmux jq fish starship")
	}
	return 0
}

func runHelperCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "tooie helper: expected subcommand: btop")
		return 2
	}
	switch strings.TrimSpace(args[0]) {
	case "btop":
		return runBtopHelperCommand(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "tooie helper: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runBtopHelperCommand(args []string) int {
	if len(args) == 0 || strings.TrimSpace(args[0]) != "setup" {
		fmt.Fprintln(os.Stderr, "tooie helper btop: expected subcommand 'setup'")
		return 2
	}
	fs := flag.NewFlagSet("helper btop setup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	runner := fs.String("runner", "auto", "")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "tooie helper btop setup: %v\n", err)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "tooie helper btop setup: unexpected arguments")
		return 2
	}
	r := normalizeRunner(*runner)
	cfg := helperConfig{Runner: r}
	if err := saveHelperConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "tooie helper btop setup: %v\n", err)
		return 1
	}

	if err := ensureTooieSupportScripts(); err != nil {
		fmt.Fprintf(os.Stderr, "tooie helper btop setup: %v\n", err)
		return 1
	}
	if _, err := os.Stat(currentBtopSetupScriptPath()); err == nil {
		cmd := exec.Command(currentBtopSetupScriptPath(), "--runner", r)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "tooie helper btop setup: helper script failed: %v\n", err)
			return 1
		}
	}
	fmt.Printf("btop helper configured (runner=%s)\n", r)
	return 0
}

func helperConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "tooie-helper.json"
	}
	return filepath.Join(home, ".config", "tooie", "helper.json")
}

func saveHelperConfig(c helperConfig) error {
	path := helperConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func yesNo(ok bool) string {
	if ok {
		return "yes"
	}
	return "no"
}
