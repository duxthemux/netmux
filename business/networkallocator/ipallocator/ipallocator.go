package ipallocator

import (
	"fmt"
	"log/slog"
	"sync"

	shell2 "github.com/duxthemux/netmux/business/shell"
)

type IPAllocator struct {
	shell shell2.Shell
	sync.Mutex
	iface      string
	freeAddrs  []string
	allocAddrs []string
}

func (i *IPAllocator) Allocate() (string, error) {
	i.Lock()
	defer i.Unlock()

	if len(i.freeAddrs) == 0 {
		return "", fmt.Errorf("no more free addresses")
	}

	addr := i.freeAddrs[0]

	i.freeAddrs = i.freeAddrs[1:]

	i.allocAddrs = append(i.allocAddrs, addr)

	err := i.shell.IfconfigAddAlias(i.iface, addr, "255.255.255.0", "10.0.0.1")
	if err != nil {
		i.freeAddrs = append(i.freeAddrs, addr)

		return "", fmt.Errorf("error adding alias: %w", err)
	}

	return addr, nil
}

func (i *IPAllocator) Release(ipAddress string) error {
	i.Lock()
	defer i.Unlock()

	err := i.shell.IfconfigRemAlias(i.iface, ipAddress)
	if err != nil {
		return fmt.Errorf("error removing alias: %w", err)
	}

	i.freeAddrs = append(i.freeAddrs, ipAddress)

	for idx, addr := range i.allocAddrs {
		if addr == ipAddress {
			i.allocAddrs[idx] = i.allocAddrs[len(i.allocAddrs)-1]
			i.allocAddrs = i.allocAddrs[:len(i.allocAddrs)-1]

			return nil
		}
	}

	return nil
}

func (i *IPAllocator) ReleaseAll(fnCleanupEach func(s string) error) error {
	for _, addr := range i.allocAddrs {
		if err := i.Release(addr); err != nil {
			return fmt.Errorf("error releasing ip address: %w", err)
		}

		if err := fnCleanupEach(addr); err != nil {
			return fmt.Errorf("error cleaning up leftovers: %w", err)
		}
	}

	return nil
}

func (i *IPAllocator) CleanUp() {
	for _, addr := range i.freeAddrs {
		err := i.Release(addr)
		if err != nil {
			slog.Debug(fmt.Sprintf("Cleanning ip - error for ip %s: %s", addr, err.Error()))
		}
	}
}

func New(iface string, cidr string) (*IPAllocator, error) {
	ret := &IPAllocator{
		shell:     shell2.New(),
		iface:     iface,
		freeAddrs: []string{},
	}

	freeAddrs, err := GetIPV4Addrs(cidr, true, true)
	if err != nil {
		return nil, fmt.Errorf("error allocating network addresses: %w", err)
	}

	ret.freeAddrs = freeAddrs

	return ret, nil
}
