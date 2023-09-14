package shell

import (
	"fmt"
	"os/exec"
)

type linuxShell struct{}

func (w *linuxShell) IfconfigAddAlias(iface string, ipaddr string, netmask string, gw string) error {
	return shStdio(fmt.Sprintf("ip addr add %s dev %s", ipaddr, iface))
}

func (w *linuxShell) IfconfigRemAlias(iface string, ipaddr string) error {
	return shStdio(fmt.Sprintf("ip addr del %s dev %s", ipaddr, iface))
}

func New() Shell {
	return &linuxShell{}
}

func sh(cmdline string) *exec.Cmd {
	cmd := exec.Command("sh", "-c", cmdline)
	return cmd
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
