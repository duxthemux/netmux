// src: https://gist.githubusercontent.com/glacjay/585620/raw/3e685b9e9b035c360afc08621a7802e16bc7add4/ping-linux.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

func main() {
	file, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		panic(fmt.Errorf("error os.Open(): %v\n", err))
	}

	ifr := make([]byte, 18)

	copy(ifr, []byte("tun0"))

	ifr[16], ifr[17] = 0x01, 0x10

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL, uintptr(file.Fd()),
		uintptr(0x400454ca), uintptr(unsafe.Pointer(&ifr[0])))
	if errno != 0 {
		panic(fmt.Errorf("error syscall.Ioctl(): %v\n", errno))
	}

	cmd := exec.Command("/sbin/ifconfig", "tun0", "192.168.7.1", "pointopoint", "192.168.7.2", "up")

	if err := cmd.Start(); err != nil {
		panic(fmt.Errorf("error running command: %v\n", err))
	}

	//------NEXT SLIDE---///

	for {
		buf := make([]byte, 2048)
		read, err := file.Read(buf)
		if err != nil {
			panic(fmt.Errorf("error os.Read(): %v\n", err))
		}

		for i := 0; i < 4; i++ {
			buf[i+12], buf[i+16] = buf[i+16], buf[i+12]
		}
		buf[20], buf[22], buf[23] = 0, 0, 0

		var checksum uint16
		for i := 20; i < read; i += 2 {
			checksum += uint16(buf[i])<<8 + uint16(buf[i+1])
		}

		checksum = ^(checksum + 4)

		buf[22], buf[23] = byte(checksum>>8), byte(checksum&((1<<8)-1))

		_, err = file.Write(buf)
		if err != nil {
			panic(fmt.Errorf("error os.Write(): %v\n", err))
		}
	}
}
