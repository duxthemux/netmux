package installer

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

const sampleSystemDUnit = `[Unit]
Description=NX Daemon

[Service]
Type=simple
User=root
Restart=always
WorkingDirectory=/srv/nx
ExecStart=/srv/nx/nx-daemon

[Install]
WantedBy=multi-user.target`

const sampleConfig = `network: 10.0.2.0/24
endpoints:
  - name: netmux 
    endpoint: netmux:50000
    kubernetes:
      config: ${USERHOME}/.kube/config
      namespace: netmux
      endpoint: netmux # netmux
      port: 50000
      context: default
#      user: ${USER}
#      kubectl: /path/to/kubectl
`

func install() error {
	userName := os.Getenv("SUDO_USER")
	if userName == "" {
		return fmt.Errorf("could not find SUDO_USER")
	}

	myUser, err := user.Lookup(userName)
	if err != nil {
		return fmt.Errorf("could not find os user for %s: %w", userName, err)
	}

	osReleaseBytes, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return fmt.Errorf("error retrieving release: %w", err)
	}

	linuxId := GetLinuxId(string(osReleaseBytes))
	linuxLikeId := GetLinuxIdLike(string(osReleaseBytes))

	dirName := "/srv/nx"

	steps := []func() error{
		execCmd("fix nx folder perms", []string{"chmod", "a+wrx", dirName}),
		execCmd("copy daemon", []string{"cp", "./nx-daemon", dirName}),
		execCmd("fix daemon perms", []string{"chmod", "u+x", filepath.Join(dirName, "nx-daemon")}),
		execCmd("copy nx", []string{"cp", "./nx", dirName}),
		execCmd("fix nx perms", []string{"chmod", "a+x", filepath.Join(dirName, "nx")}),
		execCmd("reload daemon", []string{"systemctl", "daemon-reload"}),
		execCmd("enable service", []string{"systemctl", "enable", "nx-daemon.service"}),
		execCmdSleep("enable service", []string{"systemctl", "start", "nx-daemon.service"}, 5),

		execCmd("fix perms ca.cer", []string{"chmod", "a+r", filepath.Join(dirName, "ca.cer")}),
		execCmd("fix perms ca.key", []string{"chmod", "a+r", filepath.Join(dirName, "ca.key")}),
	}

	switch {
	case linuxLikeId == "debian":
		steps = append(steps, execCmd("copy cert to trusted store", []string{"cp", filepath.Join(dirName, "ca.cer"), "/usr/local/share/ca-certificates/netmux.crt"}))
		steps = append(steps, execCmd("update certificate cache", []string{"update-ca-certificates"}))
	case linuxId == "fedora":
		steps = append(steps, execCmd("copy cert to trusted store", []string{"cp", filepath.Join(dirName, "ca.cer"), "/etc/pki/ca-trust/source/anchors/netmux.crt"}))
		steps = append(steps, execCmd("update certificate cache", []string{"update-ca-trust"}))
	default:
		return fmt.Errorf("linux distro not known: (%s/%s)", linuxId, linuxLikeId)
	}

	_, err = os.Stat(dirName)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("dir %s already exists - cant continue", dirName)
	}

	err = os.Mkdir(dirName, 0o600)
	if err != nil {
		return fmt.Errorf("could not create folder %s: %w", dirName, err)
	}

	err = os.WriteFile("/etc/systemd/system/nx-daemon.service", []byte(sampleSystemDUnit), 0o600)
	if err != nil {
		return fmt.Errorf("error generating service unit: %w", err)
	}

	configContent := strings.ReplaceAll(sampleConfig, "$USER", userName)
	configContent = strings.ReplaceAll(configContent, "$USERHOME", myUser.HomeDir)

	err = os.WriteFile(filepath.Join(dirName, "netmux.yaml"), []byte(configContent), 0o600)
	if err != nil {
		return fmt.Errorf("error creating netmux config file")
	}

	for _, step := range steps {
		err := step()
		if err != nil {
			return err
		}
	}

	log.Printf("Installation finished, please add the %s to your PATH", dirName)

	return nil
}

func uninstall() error {
	userName := os.Getenv("SUDO_USER")
	if userName == "" {
		return fmt.Errorf("could not find SUDO_USER")
	}

	steps := []func() error{
		execCmd("disable daemon", []string{"systemctl", "stop", "nx-daemon.service"}),
		execCmd("disable daemon", []string{"systemctl", "disable", "nx-daemon.service"}),
		execCmd("clean up config file", []string{"rm", "/etc/systemd/system/nx-daemon.service"}),
		execCmd("clean up files", []string{"rm", "-rf", "/srx/nx"}),
	}

	for _, step := range steps {
		err := step()
		if err != nil {
			return err
		}
	}

	log.Printf("NX uninstalled correctly.")
	return nil
}
