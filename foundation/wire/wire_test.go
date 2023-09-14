package wire_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.digitalcircle.com.br/dc/netmux/foundation/wire"
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
			args:  args{cmd: 0, pl: []byte("ASD")},
			wantW: []byte("ASD"),
		},
		{
			name:  "With Nils",
			args:  args{cmd: 0, pl: []byte("ASD\x00123")},
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

			assert.Equal(t, cmd, uint16(0))
			assert.Equal(t, aTest.wantW, pl)
		})
	}
}
