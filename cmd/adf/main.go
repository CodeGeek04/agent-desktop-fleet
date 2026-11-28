// Command adf runs a fleet of sandboxed Linux desktops for AI agents.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/host"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/runner"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

var version = "0.0.0-dev"

// app holds what commands need. It is built on first use so --help works without smolvm.
type app struct {
	smolvmPath string
	fleet      *fleet.Manager
}

func (a *app) setup() error {
	if a.fleet != nil {
		return nil
	}
	bin, err := findSmolvm(a.smolvmPath)
	if err != nil {
		return err
	}
	m := fleet.NewManager(smolvm.New(bin, runner.Exec{}), os.Stderr)
	m.Host = host.Read
	m.PauseUnsupported = pauseUnsupported()
	m.PrebuiltImage = prebuiltImage()
	if m.Limits, err = limitsFromEnv(fleet.DefaultLimits()); err != nil {
		return err
	}
	a.fleet = m
	return nil
}

// findSmolvm returns the flag value, then smolvm on PATH, then the installer's default location.
func findSmolvm(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if p, err := exec.LookPath("smolvm"); err == nil {
		return p, nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".smolvm", "smolvm")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New("smolvm not found: install it from https://github.com/smol-machines/smolvm or pass --smolvm PATH")
}

func newRootCmd() *cobra.Command {
	a := &app{}
	root := &cobra.Command{
		Use:           "adf",
		Short:         "Run a fleet of sandboxed Linux desktops for AI agents",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&a.smolvmPath, "smolvm", os.Getenv("ADF_SMOLVM"), "path to the smolvm binary (env ADF_SMOLVM)")
	root.AddCommand(lifecycleCommands(a)...)
	root.AddCommand(actionCommands(a)...)
	root.AddCommand(mcpCommand(a))
	root.AddCommand(cuaServerCommand(a))
	root.AddCommand(consoleCommand(a))
	root.AddCommand(skillCommand())
	return root
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "adf:", err)
		os.Exit(1)
	}
}
