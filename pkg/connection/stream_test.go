package connection

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

func TestResponseStream_closeWakesNext(t *testing.T) {
	tk := newTestToolkit()
	s, err := tk.OpenResponseStream("abc", make(chan struct{}), func() error { return nil })
	require.NoError(t, err)

	errc := make(chan error, 1)
	go func() {
		_, err := s.Next(context.Background())
		errc <- err
	}()
	time.Sleep(50 * time.Millisecond)
	s.Close()

	select {
	case err := <-errc:
		assert.ErrorIs(t, err, ErrStreamClosed)
	case <-time.After(5 * time.Second):
		t.Fatal("Next kept waiting after Close")
	}
}

func TestResponseStream_connectionClosedCleanly(t *testing.T) {
	tk := newTestToolkit()
	closed := make(chan struct{})
	s, err := tk.OpenResponseStream("abc", closed, func() error { return nil })
	require.NoError(t, err)
	defer s.Close()

	close(closed)
	_, err = s.Next(context.Background())
	assert.ErrorIs(t, err, ErrConnectionClosed, "a clean close is still an error for the reader")
}

func TestResponseStream_idsAreExclusive(t *testing.T) {
	tk := newTestToolkit()
	s, err := tk.OpenResponseStream("abc", make(chan struct{}), func() error { return nil })
	require.NoError(t, err)

	_, err = tk.CreateResponseChannel("abc")
	assert.ErrorIs(t, err, constants.ErrIDInUse, "a call cannot take the ID of an open stream")

	s.Close()
	_, err = tk.CreateResponseChannel("abc")
	assert.NoError(t, err, "the ID is free once the stream is closed")

	_, err = tk.OpenResponseStream("abc", make(chan struct{}), func() error { return nil })
	assert.ErrorIs(t, err, constants.ErrIDInUse, "a stream cannot take the ID of a pending call")
}

func TestClosedError(t *testing.T) {
	assert.Equal(t, ErrConnectionClosed, ClosedError(nil))
	cause := errors.New("reset by peer")
	err := ClosedError(cause)
	assert.ErrorIs(t, err, ErrConnectionClosed)
	assert.ErrorIs(t, err, cause)
}
