package networkallocator

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"go.digitalcircle.com.br/dc/netmux/business/networkallocator/dnsallocator"
	"go.digitalcircle.com.br/dc/netmux/business/networkallocator/ipallocator"
)

type NetworkAllocator struct {
	sync.Mutex
	ipAllocator  *ipallocator.IPAllocator
	dnsAllocator *dnsallocator.DNSAllocator
}

func (n *NetworkAllocator) GetIP(names ...string) (string, error) {
	n.Lock()
	defer n.Unlock()

	for _, name := range names {
		existingEntry := n.dnsAllocator.Entries().FindByName(name)
		if len(existingEntry.Names) > 0 {
			if err := n.dnsAllocator.RemoveByName(name); err != nil {
				return "", fmt.Errorf("error removing dns entry %w", err)
			}
		}
	}

	ipaddr, err := n.ipAllocator.Allocate()
	if err != nil {
		return "", fmt.Errorf("error allocating ip address: %w", err)
	}

	if err := n.dnsAllocator.Add(ipaddr, names, "name: "+strings.Join(names, ",")+" ip: "+ipaddr); err != nil {
		return "", fmt.Errorf("error allocating name: %w", err)
	}

	return ipaddr, nil
}

func (n *NetworkAllocator) ReleaseIP(ipAddress string) error {
	n.Lock()
	defer n.Unlock()

	slog.Debug("releasing ipAddress address", "ipAddress", ipAddress)

	err := n.dnsAllocator.RemoveByComment("ip: "+ipAddress, "")
	if err != nil {
		return fmt.Errorf("error removing dns entry: %w", err)
	}

	err = n.ipAllocator.Release(ipAddress)
	if err != nil {
		return fmt.Errorf("error releasing ipAddress address: %w", err)
	}

	return nil
}

func (n *NetworkAllocator) CleanUp(exception string) error {
	if err := n.dnsAllocator.CleanUp(exception); err != nil {
		return fmt.Errorf("error cleanning up dns: %w", err)
	}

	n.ipAllocator.CleanUp()

	return nil
}

func (n *NetworkAllocator) DNSEntries() []dnsallocator.DNSEntry {
	return n.dnsAllocator.Entries()
}

func New(iface string) (*NetworkAllocator, error) {
	ret := &NetworkAllocator{
		ipAllocator:  ipallocator.New(iface),
		dnsAllocator: dnsallocator.New(),
	}

	err := ret.dnsAllocator.Load()
	if err != nil {
		return nil, fmt.Errorf("error loading dns entries: %w", err)
	}

	return ret, nil
}
