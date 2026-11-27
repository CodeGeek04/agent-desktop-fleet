// Command adf runs a fleet of sandboxed Linux desktops for AI agents.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
)

var version = "0.0.0-dev"

func newRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "adf",
		Short:         "Run a fleet of sandboxed Linux desktops for AI agents",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "adf:", err)
		os.Exit(1)
	}
}
