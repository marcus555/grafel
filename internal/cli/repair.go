package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cajasmota/archigraph/internal/daemon/client"
	"github.com/cajasmota/archigraph/internal/daemon/proto"
)

// rebuild and reset both forward to the daemon's Rebuild RPC; reset
// additionally requests the daemon wipe each repo's .archigraph/ before
// indexing. The deprecated remerge alias was removed in ADR-0017 —
// callers must use `archigraph rebuild [group]` now.

func newRebuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rebuild [group] [slug]",
		Short: "Force rebuild via the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRebuildClient(cmd, args, false)
		},
	}
}

func newResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset [group] [slug]",
		Short: "Wipe .archigraph/ and rebuild via the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRebuildClient(cmd, args, true)
		},
	}
}

func runRebuildClient(cmd *cobra.Command, args []string, wipe bool) error {
	if len(args) == 0 {
		return errors.New("supply [group] (and optional [slug])")
	}
	c, err := client.Dial()
	if err != nil {
		if errors.Is(err, client.ErrDaemonNotRunning) {
			return errDaemonNotRunning
		}
		return err
	}
	defer c.Close()
	group := args[0]
	slug := ""
	if len(args) > 1 {
		slug = args[1]
	}
	reply, err := c.Rebuild(proto.RebuildArgs{Group: group, Slug: slug, Wipe: wipe})
	if err != nil {
		return err
	}
	for _, r := range reply.Repos {
		fmt.Fprintf(cmd.OutOrStdout(), "rebuilt %s\n", r)
	}
	if reply.Warning != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", reply.Warning)
	}
	return nil
}
