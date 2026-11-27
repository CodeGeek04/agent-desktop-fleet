package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/httpauth"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/mcpserver"
)

func mcpCommand(a *app) *cobra.Command {
	var httpAddr, token string
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve MCP tools for agents over stdio (default) or HTTP",
		Long: "Serve MCP tools that create, drive and delete desktops.\n\n" +
			"Stdio (Claude Code, Cursor and other local clients):\n" +
			"  claude mcp add adf -- /path/to/adf mcp\n\n" +
			"HTTP (remote clients): adf mcp --http 127.0.0.1:7400\n" +
			"Requests need 'Authorization: Bearer TOKEN'. Without --token or ADF_MCP_TOKEN a\n" +
			"random token is generated and printed to stderr.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			server := mcpserver.New(a.fleet, mcpserver.Options{Version: version, Settle: mcpserver.DefaultSettle})
			if httpAddr == "" {
				return server.Run(cmd.Context(), &mcp.StdioTransport{})
			}
			return serveHTTP(cmd.Context(), server, httpAddr, token)
		},
	}
	cmd.Flags().StringVar(&httpAddr, "http", "", "serve streamable HTTP on this address, such as 127.0.0.1:7400")
	cmd.Flags().StringVar(&token, "token", os.Getenv("ADF_MCP_TOKEN"), "bearer token for HTTP (env ADF_MCP_TOKEN)")
	return cmd
}

func serveHTTP(ctx context.Context, server *mcp.Server, addr, token string) error {
	if token == "" {
		t, err := httpauth.NewToken()
		if err != nil {
			return err
		}
		token = t
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true})
	mux := http.NewServeMux()
	mux.Handle("/mcp", httpauth.RequireToken(token, handler))

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "adf MCP endpoint: http://%s/mcp\nAuthorization: Bearer %s\n", ln.Addr(), token)

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
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
}
