package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/console"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/httpauth"
)

func consoleCommand(a *app) *cobra.Command {
	var addr string
	var noOpen bool
	cmd := &cobra.Command{
		Use:   "console",
		Short: "Open the web console: every desktop live, takeover and agent activity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			token, err := httpauth.NewToken()
			if err != nil {
				return err
			}
			idle, err := idleFromEnv()
			if err != nil {
				return err
			}
			opts := console.Options{Token: token, IdlePause: idle}
			if exe, err := os.Executable(); err == nil {
				opts.AdfPath = exe
			}
			if log := activityLog(); log != nil {
				opts.ActivityPath = log.Path
				opts.Recorder = log
			}
			srv := console.New(a.fleet, opts)

			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("listen on %s: %w (pass --addr to use another port)", addr, err)
			}
			url := fmt.Sprintf("http://%s/?t=%s", ln.Addr(), token)
			fmt.Fprintf(cmd.OutOrStdout(), "adf console: %s\nPress Ctrl+C to stop.\n", url)
			if host, _, _ := net.SplitHostPort(ln.Addr().String()); !net.ParseIP(host).IsLoopback() {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: the console is reachable from other machines; anyone with the link controls your desktops")
			}
			if !noOpen {
				openBrowser(url)
			}

			ctx := cmd.Context()
			go srv.RunCollector(ctx)
			go runIdlePauser(ctx, a.fleet, opts.ActivityPath, idle)
			httpSrv := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = httpSrv.Shutdown(shutdownCtx)
			}()
			if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:7300", "address to serve the console on")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "do not open a browser")
	return cmd
}

// openBrowser opens url in the default browser; failures are ignored because the URL is printed.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Start()
}
