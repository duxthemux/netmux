package installer

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

const samplePlistContents = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>EnvironmentVariables</key>
    <dict>
      <key>PATH</key>
      <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:</string>
    </dict>
    <key>UserName</key>
	<string>root</string>
	 <key>GroupName</key>
    <string>wheel</string>
    <key>Label</key>
    <string>nx-daemon</string>
    <key>Program</key>
    <string>$USERHOME/.nx/nx-daemon</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>LaunchOnlyOnce</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/nx-daemon.stdout</string>
    <key>StandardErrorPath</key>
    <string>/tmp/nx-daemon.stderr</string>
	<key>WorkingDirectory</key>
	<string>$USERHOME/.nx</string>
  </dict>
</plist>`

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

	dirName := filepath.Join(myUser.HomeDir, ".nx")

	steps := []func() error{
		execCmd("fix nx folder perms", []string{"chmod", "a+wrx", dirName}),
		execCmd("copy daemon", []string{"cp", "./nx-daemon", dirName}),
		execCmd("fix daemon perms", []string{"chmod", "u+x", filepath.Join(dirName, "nx-daemon")}),
		execCmd("copy nx", []string{"cp", "./nx", dirName}),
		execCmd("fix nx perms", []string{"chmod", "a+x", filepath.Join(dirName, "nx")}),
		execCmd("fix perms plist", []string{"chmod", "a+r", "/Library/LaunchDaemons/nx-daemon.plist"}),
		execCmdSleep("launch daemon", []string{"launchctl", "load", "-w", "/Library/LaunchDaemons/nx-daemon.plist"}, 5),
		execCmd("fix perms ca.cer", []string{"chmod", "a+r", filepath.Join(dirName, "ca.cer")}),
		execCmd("fix perms ca.key", []string{"chmod", "a+r", filepath.Join(dirName, "ca.key")}),
		execCmd("allow cert import", []string{"security", "authorizationdb", "write", "com.apple.trust-settings.admin", "allow"}),
		execCmd("install cert", []string{"security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", "/Library/Keychains/System.keychain", filepath.Join(dirName, "ca.cer")}),
		execCmd("deny cert import", []string{"security", "authorizationdb", "remove", "com.apple.trust-settings.admin"}),
	}

	_, err = os.Stat(dirName)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("dir %s already exists - cant continue", dirName)
	}

	err = os.Mkdir(dirName, 0o600)
	if err != nil {
		return fmt.Errorf("could not create folder %s: %w", dirName, err)
	}

	configContent := strings.ReplaceAll(sampleConfig, "$USER", userName)
	configContent = strings.ReplaceAll(configContent, "$USERHOME", myUser.HomeDir)

	err = os.WriteFile(filepath.Join(dirName, "netmux.yaml"), []byte(configContent), 0o600)
	if err != nil {
		return fmt.Errorf("error creating netmux config file")
	}

	plistContent := strings.ReplaceAll(samplePlistContents, "$USERHOME", myUser.HomeDir)

	err = os.WriteFile("/Library/LaunchDaemons/nx-daemon.plist", []byte(plistContent), 0o600)
	if err != nil {
		return fmt.Errorf("could not write plist file: %w", err)
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

	myUser, err := user.Lookup(userName)
	if err != nil {
		return fmt.Errorf("could not find os user for %s: %w", userName, err)
	}

	dirName := filepath.Join(myUser.HomeDir, ".nx")

	bs, err := exec.Command("launchctl", "unload", "-w", "/Library/LaunchDaemons/nx-daemon.plist").CombinedOutput()
	if err != nil {
		return fmt.Errorf("error loading plist file: %s, %w", string(bs), err)
	}

	if err = os.RemoveAll(dirName); err != nil {
		return fmt.Errorf("could not delete .nx folder: %w", err)
	}

	if err = os.Remove("/Library/LaunchDaemons/nx-daemon.plist"); err != nil {
		return fmt.Errorf("could not remove plist file: %w", err)
	}

	log.Printf("NX uninstalled correctly.")
	return nil
}
