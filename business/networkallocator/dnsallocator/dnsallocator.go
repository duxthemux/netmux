package dnsallocator

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type DNSEntry struct {
	Addr    string
	Names   []string
	Comment string
}

type DNSEntries []DNSEntry

const DefaultFilePerm = 0o600

func (d DNSEntries) FindByIP(ip string) DNSEntry {
	for _, v := range d {
		if v.Addr == ip {
			return v
		}
	}

	return DNSEntry{}
}

func (d DNSEntries) FindByName(name string) DNSEntry {
	for _, v := range d {
		for _, n := range v.Names {
			if n == name {
				return v
			}
		}
	}

	return DNSEntry{}
}

func (e *DNSEntry) Equals(dnsEntry DNSEntry) bool {
	if e.Addr != dnsEntry.Addr {
		return false
	}

	if len(e.Names) != len(dnsEntry.Names) {
		return false
	}

	for i := range e.Names {
		if e.Names[i] != dnsEntry.Names[i] {
			return false
		}
	}

	return e.Comment == dnsEntry.Comment
}

func (e *DNSEntry) String() string {
	hosts := strings.Join(e.Names, " ")

	return fmt.Sprintf(`%s %s #%s`, e.Addr, hosts, e.Comment)
}

func (e *DNSEntry) Load(s string) {
	parts := strings.Fields(s)
	if len(parts) < 1 {
		return
	}

	e.Addr = parts[0]

	for index := 1; index < len(parts); index++ {
		if strings.HasPrefix(parts[index], "#") {
			parts[index] = parts[index][1:]
			comment := strings.Join(parts[index:], " ")
			e.Comment = comment

			return
		}

		e.Names = append(e.Names, parts[index])
	}
}

func (e *DNSEntry) CommentMatches(s string) bool {
	return strings.Contains(e.Comment, s)
}

type DNSAllocator struct {
	fname   string
	entries DNSEntries
}

func (m *DNSAllocator) LoadBytes(bs []byte) {
	fileScanner := bufio.NewScanner(bytes.NewReader(bs))
	for fileScanner.Scan() {
		if len(fileScanner.Text()) < 1 {
			continue
		}

		hosyEntry := DNSEntry{}

		hosyEntry.Load(fileScanner.Text())
		m.entries = append(m.entries, hosyEntry)
	}
}

func (m *DNSAllocator) Bytes() []byte {
	buf := &bytes.Buffer{}

	for _, e := range m.entries {
		l := e.String()
		buf.WriteString(l)
		buf.WriteString("\n")
	}

	return buf.Bytes()
}

func (m *DNSAllocator) RemoveByComment(comment string, exception string) error {
	var hostEntries []DNSEntry

	err := m.Load()
	if err != nil {
		return fmt.Errorf("failed to load hosts file: %w", err)
	}

	for _, entry := range m.entries {
		for _, n := range entry.Names {
			if n == exception {
				hostEntries = append(hostEntries, entry)

				goto leaveLoop
			}
		}

		if !entry.CommentMatches(comment) {
			hostEntries = append(hostEntries, entry)
		}

	leaveLoop:
	}

	m.entries = hostEntries

	return m.unSyncSave()
}

func (m *DNSAllocator) RemoveByName(name string) error {
	var hostEntries []DNSEntry //nolint:prealloc

	err := m.Load()
	if err != nil {
		return fmt.Errorf("failed to load hosts file: %w", err)
	}

	found := false

	for _, entry := range m.entries {
		for _, n := range entry.Names {
			if n == name {
				slog.Debug(fmt.Sprintf("Removing hosts entry: %s", entry.String()))

				found = true

				break
			}
		}

		hostEntries = append(hostEntries, entry)
	}

	if found {
		m.entries = hostEntries

		return m.unSyncSave()
	}

	return nil
}

func (m *DNSAllocator) Equals(dnsAllocator *DNSAllocator) bool {
	if len(m.entries) != len(dnsAllocator.entries) {
		return false
	}

	for i := range m.entries {
		if !m.entries[i].Equals(dnsAllocator.entries[i]) {
			return false
		}
	}

	return true
}

func (m *DNSAllocator) Load() error {
	slog.Debug(fmt.Sprintf("Loading hosts from %s", m.fname))
	fileBytes, err := os.ReadFile(m.fname)

	switch {
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("failed to read hosts file: %w", err)
	case err != nil && errors.Is(err, os.ErrNotExist):
		return nil
	default:
		m.entries = make([]DNSEntry, 0)
		m.LoadBytes(fileBytes)

		return nil
	}
}

func (m *DNSAllocator) unSyncSave() error {
	// slog.Debug(fmt.Sprintff("Saving hosts to %s", m.fname)
	err := os.WriteFile(m.fname, m.Bytes(), DefaultFilePerm)
	if err != nil {
		panic(err)
	}

	return nil
}

func (m *DNSAllocator) Add(adr string, names []string, comment string) error {
	err := m.Load()
	if err != nil {
		return fmt.Errorf("failed to load hosts file: %w", err)
	}

	entry := DNSEntry{
		Addr:    adr,
		Names:   names,
		Comment: "src: netmux " + comment,
	}

	slog.Debug(fmt.Sprintf("Adding hosts entry: %s", entry.String()))

	m.entries = append(m.entries, entry)

	return m.unSyncSave()
}

func (m *DNSAllocator) CleanUp(exception string) error {
	return m.RemoveByComment("src: netmux", exception)
}

func (m *DNSAllocator) Entries() DNSEntries {
	return m.entries
}

type Opts func(h *DNSAllocator)

//nolint:gochecknoglobals
var WithFile = func(f string) func(h *DNSAllocator) {
	return func(h *DNSAllocator) {
		h.fname = f
	}
}

func New(opts ...Opts) *DNSAllocator {
	ret := new(DNSAllocator)
	ret.fname = Fname

	for _, o := range opts {
		o(ret)
	}

	return ret
}
