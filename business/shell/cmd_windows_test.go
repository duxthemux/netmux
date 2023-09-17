//go:build windows

package shell

import "testing"

func TestSH(t *testing.T) {
	ret, err := shStr("echo 123")
	if err != nil {
		t.Fatal(err)
	}
	if ret != "123\r\n" {
		t.Fatal("should be 123")
	}
}

func TestWinShell_IfconfigAddAlias(t *testing.T) {
	s := New()

	err := s.IfconfigAddAlias("LB", "10.0.0.1", "255.255.255.0", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	err = s.IfconfigAddAlias("LB", "10.0.0.2", "255.255.255.0", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	err = s.IfconfigAddAlias("LB", "10.0.0.3", "255.255.255.0", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	err = s.IfconfigRemAlias("LB", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	err = s.IfconfigRemAlias("LB", "10.0.0.2")
	if err != nil {
		t.Fatal(err)
	}
	err = s.IfconfigRemAlias("LB", "10.0.0.3")
	if err != nil {
		t.Fatal(err)
	}
}
