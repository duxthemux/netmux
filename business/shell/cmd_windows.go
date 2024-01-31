package shell

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	su "github.com/nyaosorg/go-windows-su"
)

type winShell struct{}

func (w *winShell) IfconfigAddAlias(iface string, ipaddr string, netmask string, gw string) error {
	err := shStdio(fmt.Sprintf("netsh interface ip add address %s %s %s", iface, ipaddr, netmask))
	if err != nil {
		return err
	}
	time.Sleep(time.Second * 5)
	return nil
}

func (w *winShell) IfconfigRemAlias(iface string, ipaddr string) error {
	return shStdio(fmt.Sprintf("netsh interface ip delete address %s %s", iface, ipaddr))
}

func (w *winShell) CmdAs(ctx context.Context, user string) (io.Writer, error) {
	cmd := exec.CommandContext(ctx, "runas", "/user:"+user, "cmd")
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
	return &winShell{}
}

func sh(cmdline string) *exec.Cmd {
	cmd := exec.Command("cmd", "/c", cmdline)
	return cmd
}

func shStr(cmdline string) (string, error) {
	cmd := sh(cmdline)

	bs, err := cmd.CombinedOutput()

	return string(bs), err
}

func shSu(cmdline string) error {
	_, err := su.ShellExecute(su.RUNAS,
		"cmd",
		"/c",
		cmdline)
	return err
}

func getUnderlingUser() (*user.User, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}
	if usr.Username == "root" {
		under := os.Getenv("SUDO_USER")
		if under != "" {
			usr, err := user.Lookup(under)
			if err != nil {
				return nil, fmt.Errorf("failed to lookup user %s: %w", under, err)
			}
			return usr, nil
		}
	}
	return usr, nil
}

func IfconfigAddAlias(iface string, ipaddr string) error {
	return shStdio(fmt.Sprintf("ifconfig %s alias %s", iface, ipaddr))
}

func IfconfigRemAlias(iface string, ipaddr string) error {
	return shStdio(fmt.Sprintf("ifconfig %s -alias %s", iface, ipaddr))
}

func LoggedUser() (string, error) {
	ret, err := shStr("who | grep console")
	if err != nil {
		return "", err
	}
	parts := strings.Split(ret, " ")
	return parts[0], nil
}

func KubeCtlKillAll() error {
	return shStdio("killall -9 kubectl")
}

func KillbyPid(p int) error {
	return shStdio(fmt.Sprintf("kill -9 %v", p))
}

func LsofTcpConnsbyPid(p int) (string, error) {
	return shStr(fmt.Sprintf("lsof -p %v", p))
}

func LaunchCtlInstallDaemon() (string, error) {
	return shStr("launchctl load /Library/LaunchDaemons/nx.plist")
}

func LaunchCtlInstallTrayAgent() (string, error) {
	usr, err := getUnderlingUser()
	if err != nil {
		return "", err
	}
	return shStr(fmt.Sprintf("launchctl load user/%s %s/Library/LaunchAgents/nx.tray.plist", usr.Uid, usr.HomeDir))
}

func LaunchCtlEnableTrayAgent() (string, error) {
	usr, err := getUnderlingUser()
	if err != nil {
		return "", err
	}
	return shStr(fmt.Sprintf("launchctl enable user/%s/nx.tray", usr.Uid))
}

func LaunchCtlUninstallTrayAgent() (string, error) {
	usr, err := getUnderlingUser()
	if err != nil {
		return "", err
	}
	return shStr(fmt.Sprintf("launchctl unload user/%s %s/Library/LaunchAgents/nx.tray.plist", usr.Uid, usr.HomeDir))
}

func LaunchCtlDisableTrayAgent() (string, error) {
	usr, err := getUnderlingUser()
	if err != nil {
		return "", err
	}
	return shStr(fmt.Sprintf("launchctl disable user/%s/nx.tray", usr.Uid))
}

func LaunchCtlStartDaemon() (string, error) {
	return shStr("launchctl start nx")
}

func LaunchCtlStartTrayAgent() (string, error) {
	return shStr("launchctl start nx")
}

func LaunchCtlStopDaemon() (string, error) {
	return shStr("launchctl stop nx")
}

func LaunchCtlUnistallDaemon() (string, error) {
	return shStr("launchctl unload /Library/LaunchDaemons/nx.plist")
}

func KillallKubectl() (string, error) {
	return shStr("killall -9 kubectl")
}

func Ping(h string) (string, error) {
	return shStr(fmt.Sprintf("ping %s", h))
}

func Nmap(h string) (string, error) {
	return shStr(fmt.Sprintf("nmap %s", h))
}

func Nc(h string) (string, error) {
	return shStr(fmt.Sprintf("nc %s", h))
}
