package ipallocator

import (
	"fmt"
	"strconv"
	"strings"
)

// GetIPV4Addrs will return a slice of strings, each one is an IP Address, part of the CIDR described.
func GetIPV4Addrs(net string, skipGw bool, skipNetwork bool) ([]string, error) {
	net = strings.TrimSpace(net)
	parts := strings.Split(net, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("cidr %s is not in the format A.B.C.D/MASK", net)
	}

	base := parts[0]
	maskStr := parts[1]

	mask, err := strconv.Atoi(maskStr)
	if err != nil {
		return nil, fmt.Errorf("mask is not int: %s", err)
	}

	if mask > 32 {
		return nil, fmt.Errorf("mask cannot be bigger than 32")
	}

	parts = strings.Split(base, ".")
	if len(parts) != 4 {
		return nil, fmt.Errorf("could not find 4 octets in %s", base)
	}

	a, b, c, d := parts[0], parts[1], parts[2], parts[3]

	aint, err := strconv.Atoi(a)
	if err != nil {
		return nil, fmt.Errorf("error parsing octet A(%s): %w", a, err)
	}

	bint, err := strconv.Atoi(b)
	if err != nil {
		return nil, fmt.Errorf("error parsing octet B(%s): %w", b, err)
	}

	cint, err := strconv.Atoi(c)
	if err != nil {
		return nil, fmt.Errorf("error parsing octet C(%s): %w", c, err)
	}

	dint, err := strconv.Atoi(d)
	if err != nil {
		return nil, fmt.Errorf("error parsing octet D(%s): %w", d, err)
	}

	ret := make([]string, 0)

	for count := 1 << (32 - mask); count > 0; count-- {

		if aint > 255 {
			return nil, fmt.Errorf("invalid entry")
		}

		if (dint != 0 || !skipGw) && (dint != 255 || !skipNetwork) {
			ret = append(ret, fmt.Sprintf("%v.%v.%v.%v", aint, bint, cint, dint))
		}

		dint++
		if dint > 255 {
			dint = 0
			cint++
			if cint > 255 {
				cint = 0
				bint++
				if bint > 255 {
					aint++
					bint = 0
				}
			}
		}

	}

	return ret, nil
}
