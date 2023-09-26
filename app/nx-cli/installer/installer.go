package installer

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

type Installer struct {
}

func (i *Installer) Install() error {
	return install()
}

func (i *Installer) Uninstall() error {
	return uninstall()
}

func New() *Installer {
	return &Installer{}
}

func execCmd(step string, cmd []string) func() error {
	return func() error {
		log.Printf("Executing %s: %s", step, strings.Join(cmd, " "))
		bs, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("could execute %s: %s, %w", step, string(bs), err)
		}
		return nil
	}
}
func execCmdSleep(step string, cmd []string, n int) func() error {
	return func() error {

		bs, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("could execute %s: %s, %w", step, string(bs), err)
		}

		time.Sleep(time.Second * time.Duration(n))
		return nil
	}
}
