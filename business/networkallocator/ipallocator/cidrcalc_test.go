package ipallocator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.digitalcircle.com.br/dc/netmux/business/networkallocator/ipallocator"
)

func TestGetAddrs(t *testing.T) {
	strs, err := ipallocator.GetIPV4Addrs("10.0.0.0/31", false, false)
	assert.NoError(t, err)
	assert.Equal(t, len(strs), 2)
	assert.Equal(t, strs[1], "10.0.0.1")
}

func TestGetAddrsNM24(t *testing.T) {
	strs, err := ipallocator.GetIPV4Addrs("10.0.0.0/24", false, false)
	assert.NoError(t, err)
	assert.Equal(t, len(strs), 256)
	assert.Equal(t, strs[1], "10.0.0.1")
}

func TestGetAddrsNM23(t *testing.T) {
	strs, err := ipallocator.GetIPV4Addrs("10.0.0.0/23", false, false)
	assert.NoError(t, err)
	assert.Equal(t, len(strs), 512)
	assert.Equal(t, strs[1], "10.0.0.1")
}

func TestGetAddrsNM23WoGateways(t *testing.T) {
	strs, err := ipallocator.GetIPV4Addrs("10.0.0.0/23", true, false)
	assert.NoError(t, err)
	assert.Equal(t, len(strs), 510)
	assert.Equal(t, strs[0], "10.0.0.1")
}

func TestGetAddrsNM23WoNetwork(t *testing.T) {
	strs, err := ipallocator.GetIPV4Addrs("10.0.0.0/23", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(strs), 510)
	assert.Equal(t, strs[0], "10.0.0.0")
	assert.Equal(t, strs[255], "10.0.1.0")
	assert.Equal(t, strs[509], "10.0.1.254")
}

func TestGetAddrsNM23WoNetworkNorGW(t *testing.T) {
	strs, err := ipallocator.GetIPV4Addrs("10.0.0.0/23", true, true)
	assert.NoError(t, err)
	assert.Equal(t, len(strs), 508)
	assert.Equal(t, strs[0], "10.0.0.1")
	assert.Equal(t, strs[254], "10.0.1.1")
	assert.Equal(t, strs[507], "10.0.1.254")
}
