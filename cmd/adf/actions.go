package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/computer"
)

func actionCommands(a *app) []*cobra.Command {
	var output string
	screenshot := &cobra.Command{
		Use:   "screenshot NAME",
		Short: "Save a PNG screenshot of a desktop",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			png, err := a.fleet.Computer(args[0]).Screenshot(cmd.Context())
			if err != nil {
				return err
			}
			path := output
			if path == "" {
				path = args[0] + ".png"
			}
			if err := os.WriteFile(path, png, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%d bytes)\n", path, len(png))
			return nil
		},
	}
	screenshot.Flags().StringVarP(&output, "output", "o", "", "file to write (default NAME.png)")

	var button string
	var double bool
	click := &cobra.Command{
		Use:   "click NAME X Y",
		Short: "Click at a point",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := parseInts(args[1], args[2])
			if err != nil {
				return err
			}
			if err := a.setup(); err != nil {
				return err
			}
			c := a.fleet.Computer(args[0])
			if double {
				return c.DoubleClick(cmd.Context(), p[0], p[1], computer.Button(button))
			}
			return c.Click(cmd.Context(), p[0], p[1], computer.Button(button))
		},
	}
	click.Flags().StringVar(&button, "button", "left", "left, middle or right")
	click.Flags().BoolVar(&double, "double", false, "double-click")

	move := &cobra.Command{
		Use:   "move NAME X Y",
		Short: "Move the pointer",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := parseInts(args[1], args[2])
			if err != nil {
				return err
			}
			if err := a.setup(); err != nil {
				return err
			}
			return a.fleet.Computer(args[0]).Move(cmd.Context(), p[0], p[1])
		},
	}

	drag := &cobra.Command{
		Use:   "drag NAME X1 Y1 X2 Y2",
		Short: "Drag with the left button from one point to another",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := parseInts(args[1:]...)
			if err != nil {
				return err
			}
			if err := a.setup(); err != nil {
				return err
			}
			return a.fleet.Computer(args[0]).Drag(cmd.Context(), p[0], p[1], p[2], p[3])
		},
	}

	var dir string
	var amount int
	scroll := &cobra.Command{
		Use:   "scroll NAME X Y",
		Short: "Scroll the wheel at a point",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := parseInts(args[1], args[2])
			if err != nil {
				return err
			}
			if err := a.setup(); err != nil {
				return err
			}
			return a.fleet.Computer(args[0]).Scroll(cmd.Context(), p[0], p[1], computer.Direction(dir), amount)
		},
	}
	scroll.Flags().StringVar(&dir, "dir", "down", "up, down, left or right")
	scroll.Flags().IntVar(&amount, "amount", 3, "wheel clicks")

	typeCmd := &cobra.Command{
		Use:   "type NAME TEXT",
		Short: "Type text",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			return a.fleet.Computer(args[0]).Type(cmd.Context(), args[1])
		},
	}

	key := &cobra.Command{
		Use:   "key NAME KEYS",
		Short: `Press a key or chord, such as "Return" or "ctrl+l"`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			return a.fleet.Computer(args[0]).Key(cmd.Context(), args[1])
		},
	}

	cursor := &cobra.Command{
		Use:   "cursor NAME",
		Short: "Print the pointer position",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			x, y, err := a.fleet.Computer(args[0]).CursorPosition(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d %d\n", x, y)
			return nil
		},
	}

	execCmd := &cobra.Command{
		Use:   "exec NAME -- COMMAND [ARGS...]",
		Short: "Run a command inside a desktop and print its output",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			out, err := a.fleet.Exec(cmd.Context(), args[0], args[1:]...)
			if _, werr := cmd.OutOrStdout().Write(out); werr != nil && err == nil {
				err = werr
			}
			return err
		},
	}

	return []*cobra.Command{screenshot, click, move, drag, scroll, typeCmd, key, cursor, execCmd}
}
