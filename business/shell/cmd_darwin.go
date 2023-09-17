package shell

import (
	"fmt"
	"os/exec"
)

type darwinShell struct{}

func (w *darwinShell) IfconfigAddAlias(iface string, ipaddr string, _ string, _ string) error {
	return shStdio(fmt.Sprintf("ifconfig %s alias %s", iface, ipaddr))
}

func (w *darwinShell) IfconfigRemAlias(iface string, ipaddr string) error {
	return shStdio(fmt.Sprintf("ifconfig %s -alias %s", iface, ipaddr))
}

//nolint:ireturn,nolintlint
func New() Shell {
	return &darwinShell{}
}

func sh(cmdline string) *exec.Cmd {
	cmd := exec.Command("sh", "-c", cmdline)

	return cmd
}
