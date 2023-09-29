package shell

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type darwinShell struct{}

func (w *darwinShell) IfconfigAddAlias(iface string, ipaddr string, _ string, _ string) error {
	return shStdio(fmt.Sprintf("ifconfig %s alias %s", iface, ipaddr))
}

func (w *darwinShell) IfconfigRemAlias(iface string, ipaddr string) error {
	return shStdio(fmt.Sprintf("ifconfig %s -alias %s", iface, ipaddr))
}

func (w *darwinShell) CmdAs(ctx context.Context, user string) (io.Writer, error) {

	cmd := exec.CommandContext(ctx, "su", "-", user)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	writer, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("error piping stdin: %w", err)
	}

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting port forward command: %w", err)
	}

	return writer, nil

}

//nolint:ireturn,nolintlint
func New() Shell {
	return &darwinShell{}
}

func sh(cmdline string) *exec.Cmd {
	cmd := exec.Command("sh", "-c", cmdline)

	return cmd
}
