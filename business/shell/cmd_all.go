package shell

import (
	"context"
	"fmt"
	"io"
	"os"
)

func shStdio(cmdline string) error {
	cmd := sh(cmdline)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error running %s: %w", cmdline, err)
	}

	return nil
}

type Shell interface {
	IfconfigAddAlias(iface string, ipaddr string, netmask string, gw string) error
	IfconfigRemAlias(iface string, ipaddr string) error
	CmdAs(ctx context.Context, user string) (io.Writer, error)
}
