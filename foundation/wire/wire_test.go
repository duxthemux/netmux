package wire_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/duxthemux/netmux/foundation/wire"
)

//nolint:paralleltest
func TestWireProto(t *testing.T) {
	aWire := wire.Wire{}

	type args struct {
		cmd uint16
		pl  []byte
	}

	tests := []struct {
		name    string
		args    args
		wantW   []byte
		wantErr bool
	}{
		{
			name:  "Simple",
			args:  args{cmd: 1, pl: []byte("ASD")},
			wantW: []byte("ASD"),
		},
		{
			name:  "With Nils",
			args:  args{cmd: 1, pl: []byte("ASD\x00123")},
			wantW: []byte("ASD\x00123"),
		},
	}
	for _, aTest := range tests {
		t.Run(aTest.name, func(t *testing.T) {
			w := &bytes.Buffer{}

			err := aWire.Write(w, aTest.args.cmd, aTest.args.pl)
			assert.NoError(t, err)

			cmd, pl, err := aWire.Read(w)
			assert.NoError(t, err)

			assert.Equal(t, cmd, uint16(1))
			assert.Equal(t, aTest.wantW, pl)
		})
	}
}
