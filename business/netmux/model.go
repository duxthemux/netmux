package netmux

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	DirectionL2C = "L2C"
	DirectionC2L = "C2L"

	FamilyTCP = "tcp"
	FamilyUpd = "upd"

	EventBridgeAdd = "bridge-add"
	EventBridgeDel = "bridge-del"
	EventBridgeUp  = "bridge-up"
)

func CmdToString(cmdUint16 uint16) string {
	switch cmdUint16 {
	case CmdUnknown:
		return "Unknown"
	case CmdControl:
		return "Control"
	case CmdProxy:
		return "proxy"
	case CmdEvents:
		return "events"
	case CmdRevProxyListen:
		return "revproxy-listen"
	case CmdRevProxyWork:
		return "revproxy-work"
	default:
		return fmt.Sprintf("code %d now known", cmdUint16)
	}
}

type Bridge struct {
	Namespace     string `json:"namespace"               yaml:"namespace"`
	Name          string `json:"name,omitempty"          yaml:"name"`
	LocalAddr     string `json:"localAddr,omitempty"     yaml:"localAddr"`
	LocalPort     string `json:"localPort,omitempty"     yaml:"localPort"`
	ContainerAddr string `json:"containerAddr,omitempty" yaml:"containerAddr"`
	ContainerPort string `json:"containerPort,omitempty" yaml:"containerPort"`
	Direction     string `json:"direction,omitempty"     yaml:"direction"`
	Family        string `json:"family,omitempty"        yaml:"family"`
}

func (b *Bridge) FullLocalAddr() string {
	return fmt.Sprintf("%s:%s", b.LocalAddr, b.LocalPort)
}

func (b *Bridge) FullContainerAddr() string {
	return fmt.Sprintf("%s:%s", b.ContainerAddr, b.ContainerPort)
}

func (b *Bridge) LocalName() string {
	if b.Namespace != "" {
		return fmt.Sprintf("%s.%s", b.Name, b.Namespace)
	}

	return b.Name
}

func (b *Bridge) Validate() error {
	if b.Name == "" {
		return fmt.Errorf("invalid name")
	}

	if b.LocalAddr == "" {
		return fmt.Errorf("invalid local address")
	}

	if b.LocalPort == "" {
		return fmt.Errorf("invalid local port")
	}

	if b.ContainerAddr == "" {
		return fmt.Errorf("invalid container address")
	}

	if b.ContainerPort == "" {
		return fmt.Errorf("invalid container port")
	}

	if b.Direction != DirectionL2C && b.Direction != DirectionC2L {
		return fmt.Errorf("invalid direction")
	}

	if b.Family != FamilyTCP && b.Family != FamilyUpd {
		return fmt.Errorf("invalid family")
	}

	return nil
}

func (b *Bridge) String() string {
	return fmt.Sprintf("%v: %v %v=>%v %v", b.Name, b.Family, b.FullLocalAddr(), b.FullContainerAddr(), b.Direction)
}

const (
	CmdUnknown uint16 = iota
	CmdControl
	CmdEvents
	CmdProxy
	CmdRevProxyListen
	CmdRevProxyWork
)

type Message struct {
	ID      int64  `json:"id"`
	ReplyTo int64  `json:"replyTo"`
	Err     string `json:"err"`
}

type CmdRawResponse struct {
	Cmd uint16
	Pl  []byte
}

func (c *CmdRawRequest) Read(res any) error {
	if err := json.Unmarshal(c.Pl, res); err != nil {
		return fmt.Errorf("error unmarshalling cmd: %w", err)
	}

	return nil
}

type CmdRawRequest struct {
	Cmd uint16
	Pl  []byte
}

func (c *CmdRawRequest) Write(req any) error {
	bs, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("error unmarshalling cmd: %w", err)
	}

	c.Pl = bs

	return nil
}

type CmdConnControlRequest struct {
	Message
}
type CmdConnControlResponse struct {
	Message
}

type NoopMessage struct {
	Message
}

type PingRequest struct {
	Message
	CreatedAt time.Time `json:"createdAt"`
}

type PingResponse struct {
	Message
	CreatedAt time.Time `json:"createdAt"`
	RepliedAt time.Time `json:"repliedAtt"`
}

type ProxyRequest struct {
	Message  `json:"message"`
	Name     string `json:"name" yaml:"name"`
	Family   string `json:"family,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
}

type ProxyResponse struct {
	Message
}

type RevProxyListenRequest struct {
	Message
	Name       string `json:"name" yaml:"name"`
	Family     string `json:"family,omitempty"`
	RemoteAddr string `json:"endpoint,omitempty"`
	LocalAddr  string `json:"localAddr,omitempty"`
}

type RevProxyListenResponse struct {
	Message
}

type RevProxyWRequest struct {
	Message
	Family        string `json:"family,omitempty"`
	Endpoint      string `json:"endpoint,omitempty"`
	LocalEndpoint string `json:"localEndpoint,omitempty"`
}

type RevProxyEvent struct {
	Message
	ID string `json:"id,omitempty"`
}

type RevProxyWorkRequest struct {
	Message
	ID string `json:"id,omitempty"`
}

type RevProxyWorkResponse struct {
	Message
	ID int `json:"id,omitempty"`
}

type EventRequest struct {
	Message
}

type EventResponse struct {
	Message
}

type Event struct {
	EvtName string `json:"evtName,omitempty"`
	Bridge  Bridge `json:"bridge"`
}

func (e Event) String() string {
	return fmt.Sprintf("%#v", e)
}
