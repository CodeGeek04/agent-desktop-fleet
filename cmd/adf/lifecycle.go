package main

import (
	"errors"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/runner"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

func lifecycleCommands(a *app) []*cobra.Command {
	doctor := &cobra.Command{
		Use:   "doctor",
		Short: "Check that this computer can run desktops",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			healthy := true
			if bin, err := findSmolvm(a.smolvmPath); err != nil {
				fmt.Fprintln(out, "FAIL smolvm:", err)
				healthy = false
			} else if v, err := smolvm.New(bin, runner.Exec{}).Version(cmd.Context()); err != nil {
				fmt.Fprintln(out, "FAIL smolvm:", err)
				healthy = false
			} else {
				fmt.Fprintf(out, "ok   smolvm: %s (%s)\n", v, bin)
			}
			if err := checkVirtualization(); err != nil {
				fmt.Fprintln(out, "FAIL virtualization:", err)
				healthy = false
			} else {
				fmt.Fprintln(out, "ok   hardware virtualization")
			}
			if !healthy {
				return errors.New("doctor found problems")
			}
			return nil
		},
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Build and warm the desktop template (the first run takes a few minutes)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			start := time.Now()
			if err := a.fleet.EnsureTemplate(cmd.Context()); err != nil {
				return err
			}
			if err := a.fleet.WarmTemplate(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "template ready in %s; create desktops with 'adf create NAME'\n",
				time.Since(start).Round(100*time.Millisecond))
			return nil
		},
	}

	create := &cobra.Command{
		Use:   "create NAME...",
		Short: "Create running desktops branched from the warm template",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			for _, name := range args {
				start := time.Now()
				if _, err := a.fleet.CreateDesktop(cmd.Context(), name); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "created %s in %s\n", name, time.Since(start).Round(10*time.Millisecond))
			}
			return nil
		},
	}

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List desktops",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			ds, err := a.fleet.List(cmd.Context())
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSTATE")
			for _, d := range ds {
				fmt.Fprintf(w, "%s\t%s\n", d.Name, d.State)
			}
			return w.Flush()
		},
	}

	stop := &cobra.Command{
		Use:   "stop NAME...",
		Short: "Stop desktops",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			for _, name := range args {
				if err := a.fleet.Stop(cmd.Context(), name); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "stopped", name)
			}
			return nil
		},
	}

	start := &cobra.Command{
		Use:   "start NAME...",
		Short: "Start stopped desktops",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			for _, name := range args {
				begin := time.Now()
				if err := a.fleet.Start(cmd.Context(), name); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "started %s in %s\n", name, time.Since(begin).Round(10*time.Millisecond))
			}
			return nil
		},
	}

	pause := &cobra.Command{
		Use:   "pause NAME...",
		Short: "Pause desktops: free their memory and keep everything; any action resumes them",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			for _, name := range args {
				how, err := a.fleet.Sleep(cmd.Context(), name)
				if err != nil {
					return err
				}
				if how == "stopped" {
					fmt.Fprintf(cmd.OutOrStdout(), "stopped %s (pausing is not available here: %s)\n", name, a.fleet.PauseUnsupported)
					continue
				}
				fmt.Fprintln(cmd.OutOrStdout(), "paused", name)
			}
			return nil
		},
	}

	status := &cobra.Command{
		Use:   "status",
		Short: "Show this computer's room for desktops and the limits in force",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			c, err := a.fleet.Capacity(cmd.Context())
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if c.HostKnown {
				fmt.Fprintf(out, "Memory: %s of %s available, %d CPUs, load %.1f\n",
					fleet.HumanBytes(c.Host.AvailableMemBytes), fleet.HumanBytes(c.Host.TotalMemBytes), c.Host.CPUs, c.Host.Load1)
			}
			fmt.Fprintf(out, "Desktops: %d running, %d paused, %d stopped\n", c.Running, c.Paused, c.Stopped)
			runCap := "no cap on running desktops"
			if c.Limits.MaxRunning > 0 {
				runCap = fmt.Sprintf("at most %d running", c.Limits.MaxRunning)
			}
			fmt.Fprintf(out, "Limits: %s kept for this computer, %s per desktop, %s\n",
				fleet.HumanBytes(c.Limits.ReserveBytes), fleet.HumanBytes(c.Limits.DesktopBytes), runCap)
			if c.MoreThatFit >= 0 {
				fmt.Fprintf(out, "Room for about %d more desktops\n", c.MoreThatFit)
			}
			if a.fleet.PauseUnsupported != "" {
				fmt.Fprintf(out, "Pausing is not available here (%s), so idle desktops keep running\n", a.fleet.PauseUnsupported)
			} else if idle, err := idleFromEnv(); err == nil && idle > 0 {
				fmt.Fprintf(out, "Idle desktops pause after %s (ADF_IDLE_PAUSE)\n", humanDuration(idle))
			}
			return nil
		},
	}

	rm := &cobra.Command{
		Use:   "rm NAME...",
		Short: "Stop and delete desktops",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			for _, name := range args {
				if err := a.fleet.Delete(cmd.Context(), name); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "removed", name)
			}
			return nil
		},
	}

	return []*cobra.Command{doctor, initCmd, status, create, ls, start, pause, stop, rm}
}
