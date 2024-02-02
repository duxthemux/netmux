package networkallocator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/duxthemux/netmux/business/networkallocator/dnsserver"
	"github.com/duxthemux/netmux/business/networkallocator/ipallocator"
)

type NetworkAllocator struct {
	sync.Mutex
	ipAllocator *ipallocator.IPAllocator
	dnsServer   *dnsserver.Server
}

func (n *NetworkAllocator) GetIP(names ...string) (string, error) {
	n.Lock()
	defer n.Unlock()

	for _, name := range names {
		n.dnsServer.Del(name)
	}

	ipaddr, err := n.ipAllocator.Allocate()
	if err != nil {
		return "", fmt.Errorf("error allocating ip address: %w", err)
	}

	for _, name := range names {
		n.dnsServer.Add(ipaddr, name)
	}

	return ipaddr, nil
}

func (n *NetworkAllocator) ReleaseIP(ipAddress string) error {
	n.Lock()
	defer n.Unlock()

	slog.Debug("releasing ipAddress address", "ipAddress", ipAddress)
	n.dnsServer.Del(ipAddress)

	err := n.ipAllocator.Release(ipAddress)
	if err != nil {
		return fmt.Errorf("error releasing ipAddress address: %w", err)
	}

	return nil
}

func (n *NetworkAllocator) CleanUp(exception string) error {
	n.dnsServer.Clear()

	n.ipAllocator.CleanUp()

	return nil
}

func New(ctx context.Context, iface string, cidr string) (*NetworkAllocator, error) {
	slog.Debug("Creating NWAllocator", "iface", iface, "cidr", cidr)
	myIpallocator, err := ipallocator.New(iface, cidr)
	if err != nil {
		return nil, err
	}

	ret := &NetworkAllocator{
		ipAllocator: myIpallocator,
		dnsServer:   &dnsserver.Server{},
	}

	go ret.dnsServer.ListenAndServe(ctx)

	return ret, nil
}
