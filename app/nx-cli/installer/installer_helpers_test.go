package installer_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go.digitalcircle.com.br/dc/netmux/app/nx-cli/installer"
)

const alpineOsRelease = `NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.18.3
PRETTY_NAME="Alpine Linux v3.18"
HOME_URL="https://alpinelinux.org/"
BUG_REPORT_URL="https://gitlab.alpinelinux.org/alpine/aports/-/issues"`

const ubuntuOsRelease = `PRETTY_NAME="Ubuntu 23.04"
NAME="Ubuntu"
VERSION_ID="23.04"
VERSION="23.04 (Lunar Lobster)"
VERSION_CODENAME=lunar
ID=ubuntu
ID_LIKE=debian
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
UBUNTU_CODENAME=lunar
LOGO=ubuntu-logo`

const brokenUbuntuOsRelease = `PRETTY_NAME="Ubuntu 23.04"
NAME="Ubuntu"
VERSION_ID="23.04"
VERSION="23.04 (Lunar Lobster)"
VERSION_CODENAME=lunar
_ID=ubuntu
ID_LIKE=debian
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
UBUNTU_CODENAME=lunar
LOGO=ubuntu-logo`

func TestGetLinuxId(t *testing.T) {
	id := installer.GetLinuxId(ubuntuOsRelease)
	require.Equal(t, "ubuntu", id)
	id = installer.GetLinuxIdLike(ubuntuOsRelease)
	require.Equal(t, "debian", id)
	id = installer.GetLinuxId(alpineOsRelease)
	require.Equal(t, "alpine", id)
	id = installer.GetLinuxId(brokenUbuntuOsRelease)
	require.Equal(t, "", id)
}
