// Package wire implements low level protocol handling over the wire.
// Logic is simple: every payload is composed by:
//   - int16 with the type of package
//   - int64 with the payload length
//   - []byte with the payload itself. Len([]byte) should be equal to the value expressed by int64.
package wire

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

const HeaderLen = 10

// Wire is the responsible for reading and writing low level proto to/from the wire.
type Wire struct{}

// Write will write a payload to the wire.
func (wi *Wire) Write(writer io.Writer, cmd uint16, payload []byte) error {
	header := make([]byte, HeaderLen)

	binary.LittleEndian.PutUint16(header, cmd)
	binary.LittleEndian.PutUint64(header[2:], uint64(len(payload)))

	if _, err := writer.Write(header); err != nil {
		return fmt.Errorf("error sending header: %writer", err)
	}

	if _, err := writer.Write(payload); err != nil {
		return fmt.Errorf("error sending payload: %writer", err)
	}

	return nil
}

// Read extracts next payload from the wire.
func (wi *Wire) Read(reader io.Reader) (cmd uint16, payload []byte, err error) {
	header := make([]byte, HeaderLen)
	if _, err := reader.Read(header); err != nil {
		return 0, nil, fmt.Errorf("error reading header: %w", err)
	}

	cmd = binary.LittleEndian.Uint16(header)

	plLen := binary.LittleEndian.Uint64(header[2:])

	defer func() {
		r := recover()
		if r != nil {
			// TODO: this is a short term solution to handle undesired connections.
			//  the timeout reduces overload until we find a better solution.
			time.Sleep(time.Second * 5)
			err = fmt.Errorf("wrong wire protocol format (recover): %s. %v", base64.StdEncoding.EncodeToString(header), r)
		}
	}()

	payload = make([]byte, plLen)

	if _, err := reader.Read(payload); err != nil {
		return 0, nil, fmt.Errorf("error reading payload: %w", err)
	}

	return
}

// WriteJSON adds a little to Write, allowing prompt marshalling of datastructures to the wire in Json format.
func (wi *Wire) WriteJSON(writer io.Writer, cmd uint16, pl any) error {
	bytes, err := json.Marshal(pl)
	if err != nil {
		return fmt.Errorf("WriteJsonToWire: error marshalling pl: %writer", err)
	}

	return wi.Write(writer, cmd, bytes)
}

// ReadJSON works in a similar way to WriteJSON, but reads from the wire and poupulates a data structure.
// The provided cmd prevents us from extracting data that is not the command we are waiting for.
// If multiple commands may come, please use Read and unmarshall your data manually.
func (wi *Wire) ReadJSON(reader io.Reader, cmd uint16, payload any) error {
	recvcmd, bytes, err := wi.Read(reader)
	if err != nil {
		return fmt.Errorf("ReadJsonFromWire: error reading payload: %w", err)
	}

	if cmd != recvcmd {
		return fmt.Errorf("ReadJsonFromWire: wrong command received: %v", recvcmd)
	}

	if err = json.Unmarshal(bytes, payload); err != nil {
		return fmt.Errorf("ReadJsonFromWire: error unmarshalling payload: %w", err)
	}

	return nil
}
