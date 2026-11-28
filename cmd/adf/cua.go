package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/cuaserver"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/httpauth"
)

func cuaServerCommand(a *app) *cobra.Command {
	var addr, token, desktop, prefix string
	cmd := &cobra.Command{
		Use:   "cua-server",
		Short: "Serve the Cua Cloud API so existing Cua code drives adf desktops",
		Long: "Serve the Cua computer-server API (POST /cmd) and the /v1/vms platform API for every\n" +
			"desktop on one address, so code written for Cua Cloud works after changing environment\n" +
			"variables. It prints the variables to set. Supported clients: cua-computer 0.5.19, the\n" +
			"last release, and cua-sandbox 0.8.0 or earlier (0.9.0 and later cannot reach this API).\n" +
			"Linux desktops only; no WebSocket, clipboard, window or PTY commands yet.\n\n" +
			"A request names its desktop with X-Container-Name (the Computer or Sandbox name) or the\n" +
			"/d/NAME/ path; otherwise --desktop, or the only desktop there is, is used. Every command\n" +
			"shows in 'adf console' as a Cua agent action.\n\n" +
			"Without --token or ADF_CUA_TOKEN a random token is generated and printed; a token is\n" +
			"required when listening on an address other than loopback.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if desktop != "" {
				if err := fleet.ValidateName(desktop); err != nil {
					return fmt.Errorf("--desktop: %w", err)
				}
			}
			if err := fleet.ValidateName(prefix + "a1b2c3"); err != nil {
				return fmt.Errorf("--name-prefix %q does not make valid desktop names: %w", prefix, err)
			}
			if token == "" && !isLoopback(addr) {
				return fmt.Errorf("%s is not a loopback address, so set a token with --token or ADF_CUA_TOKEN: "+
					"anyone who reaches the server can run commands as root in your desktops", addr)
			}
			if err := a.setup(); err != nil {
				return err
			}
			if token == "" {
				t, err := httpauth.NewToken()
				if err != nil {
					return err
				}
				token = t
			}
			idle, err := idleFromEnv()
			if err != nil {
				return err
			}
			opts := cuaserver.Options{Token: token, Desktop: desktop, NamePrefix: prefix, Version: version, Log: cmd.ErrOrStderr()}
			ctx := cmd.Context()
			if log := activityLog(); log != nil {
				opts.Activity = log // assigned only when non-nil, so the interface is never a typed nil
				go runIdlePauser(ctx, a.fleet, log.Path, idle)
			}
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("listen on %s: %w (pass --addr to use another port)", addr, err)
			}
			printCuaSetup(cmd.OutOrStdout(), ln.Addr().(*net.TCPAddr), token)
			if state, err := a.fleet.TemplateState(ctx); err == nil && state == "missing" {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: no desktop template yet, so the first desktop takes a few minutes; run 'adf init' first to avoid client timeouts")
			}

			srv := &http.Server{Handler: cuaserver.New(a.fleet, opts).Handler(), ReadHeaderTimeout: 10 * time.Second}
			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = srv.Shutdown(shutdownCtx)
			}()
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8000", "address to serve on (8000 is the port Cua clients default to)")
	cmd.Flags().StringVar(&token, "token", os.Getenv("ADF_CUA_TOKEN"), "API key clients send as CUA_API_KEY (env ADF_CUA_TOKEN)")
	cmd.Flags().StringVar(&desktop, "desktop", "", "desktop for requests that name none")
	cmd.Flags().StringVar(&prefix, "name-prefix", "cua-", "start of the names of desktops created through POST /v1/vms without a name")
	return cmd
}

// isLoopback reports whether addr (host:port) only accepts connections from this computer.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// printCuaSetup prints the address, the token and the environment for both Cua clients.
func printCuaSetup(w io.Writer, addr *net.TCPAddr, token string) {
	host := addr.IP.String()
	remote := addr.IP.IsUnspecified()
	if remote {
		host = "127.0.0.1"
	}
	base := "http://" + net.JoinHostPort(host, fmt.Sprint(addr.Port))
	fmt.Fprintf(w, "adf cua-server: %s (token %s)\n\n", base, token)
	fmt.Fprintf(w, "cua-computer 0.5.19, Computer(os_type=\"linux\", provider_type=\"cloud\", name=\"DESKTOP\"):\n")
	fmt.Fprintf(w, "  export CUA_API_KEY=%s\n  export CUA_API_BASE=%s\n  export CUA_API_BASE_URL=%s\n\n", token, base, base)
	fmt.Fprintf(w, "cua-sandbox 0.8.0 or earlier, Sandbox.create, Sandbox.ephemeral or Sandbox.connect:\n")
	fmt.Fprintf(w, "  export CUA_API_KEY=%s\n  export CUA_BASE_URL=%s\n\n", token, base)
	if remote {
		fmt.Fprintln(w, "From another computer, use this computer's address instead of 127.0.0.1.")
	}
	fmt.Fprintln(w, "Create desktops with 'adf create NAME' (or let cua-sandbox create them), and watch them with 'adf console'.")
	fmt.Fprintln(w, "Press Ctrl+C to stop.")
}
