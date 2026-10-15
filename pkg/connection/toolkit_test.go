package connection

import (
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestToolkit() *Toolkit {
	return &Toolkit{
		ResponseChannels:     make(map[string]chan RPCResponse[cbor.RawMessage]),
		NotificationChannels: make(map[string]chan Notification),
	}
}

// TestDeliverResponse_callerStoppedListening reproduces the state a WebSocket
// connection is in when a caller's context expires after the read loop has
// found its response channel but before it has sent on it. Nobody will ever
// receive from the channel, and delivering to it must not block.
func TestDeliverResponse_callerStoppedListening(t *testing.T) {
	tk := newTestToolkit()

	_, err := tk.CreateResponseChannel("abc")
	require.NoError(t, err)

	done := make(chan bool)
	go func() {
		done <- tk.DeliverResponse("abc", RPCResponse[cbor.RawMessage]{ID: "abc"})
	}()

	select {
	case delivered := <-done:
		assert.True(t, delivered)
	case <-time.After(time.Second):
		t.Fatal("DeliverResponse blocked on a response channel nobody reads")
	}
}

func TestDeliverResponse_deliversOnce(t *testing.T) {
	tk := newTestToolkit()

	ch, err := tk.CreateResponseChannel("abc")
	require.NoError(t, err)

	assert.True(t, tk.DeliverResponse("abc", RPCResponse[cbor.RawMessage]{ID: "abc"}))
	// A duplicate response for the same id finds no waiting caller.
	assert.False(t, tk.DeliverResponse("abc", RPCResponse[cbor.RawMessage]{ID: "abc"}))

	res := <-ch
	assert.Equal(t, "abc", res.ID)

	_, ok := tk.GetResponseChannel("abc")
	assert.False(t, ok, "a delivered response channel is removed")
}

func TestDeliverResponse_unknownID(t *testing.T) {
	tk := newTestToolkit()
	assert.False(t, tk.DeliverResponse("nope", RPCResponse[cbor.RawMessage]{ID: "nope"}))
}
