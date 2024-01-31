package shell

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type linuxShell struct{}

func (w *linuxShell) IfconfigAddAlias(iface string, ipaddr string, netmask string, gw string) error {
	return shStdio(fmt.Sprintf("ip addr add %s dev %s", ipaddr, iface))
}

func (w *linuxShell) IfconfigRemAlias(iface string, ipaddr string) error {
	return shStdio(fmt.Sprintf("ip addr del %s dev %s", ipaddr, iface))
}

func (w *linuxShell) CmdAs(ctx context.Context, user string) (io.Writer, error) {
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

func New() Shell {
	return &linuxShell{}
}

func sh(cmdline string) *exec.Cmd {
	cmd := exec.Command("sh", "-c", cmdline)
	return cmd
}

func shStr(cmdline string) (string, error) {
	cmd := sh(cmdline)

	bs, err := cmd.CombinedOutput()

	return string(bs), err
}

func Ping(h string) (string, error) {
	return shStr(fmt.Sprintf("ping -c 4 %s", h))
}

func Nmap(h string) (string, error) {
	return shStr(fmt.Sprintf("nmap %s", h))
}

func Nc(h string) (string, error) {
	return shStr(fmt.Sprintf("nc %s", h))
}
