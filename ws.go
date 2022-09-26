package surrealdb

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type WS struct {
	ws   *websocket.Conn        // websocket connection
	send chan<- *RPCRequest     // sender channel
	recv <-chan *RPCRawResponse // receive channel
	emit struct {
		// TODO: use the lock less, through smaller locks (separate once/when locks ?)
		// or ideally by removing locks altogether
		lock sync.Mutex // pause threads to avoid conflicts

		// do the callbacks really need to be a list ?
		once map[any][]func(error, any) // once listeners
		when map[any][]func(error, any) // when listeners
	}
}

func NewWebsocket(ctx context.Context, url string) (*WS, error) {
	dialer := websocket.DefaultDialer
	dialer.EnableCompression = true

	// establish connection
	so, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	ws := &WS{ws: so}

	// initilialize the callback maps here so we don't need to check them at runtime
	ws.emit.once = make(map[any][]func(error, any))
	ws.emit.when = make(map[any][]func(error, any))

	// setup loops and channels
	ws.initialise(ctx)

	return ws, nil

}

// --------------------------------------------------
// Public methods
// --------------------------------------------------

func (ws *WS) Close() error {

	msg := websocket.FormatCloseMessage(1000, "")
	return ws.ws.WriteMessage(websocket.CloseMessage, msg)

}

func (ws *WS) Send(id string, method string, params []any) {

	go func() {
		ws.send <- &RPCRequest{
			ID:     id,
			Method: method,
			Params: params,
		}
	}()

}

type responseValue struct {
	value  any
	method string
	err    error
}

// Once Subscribe to once()
func (ws *WS) Once(id, method string) <-chan responseValue {

	out := make(chan responseValue)

	ws.once(id, func(e error, r any) {
		out <- responseValue{
			value:  r,
			method: method,
			err:    e,
		}
		close(out)
	})

	return out

}

// When Subscribe to when()
func (ws *WS) When(id, method string) <-chan responseValue {
	// TODO: make this cancellable (use of context.Context ?)

	out := make(chan responseValue)

	ws.when(id, func(e error, r any) {
		out <- responseValue{
			method: method,
			value:  r,
			err:    e,
		}
	})

	return out

}

// --------------------------------------------------
// Private methods
// --------------------------------------------------

func (ws *WS) once(id any, fn func(error, any)) {

	// pauses traffic in others threads, so we can add the new listener without conflicts

	ws.emit.lock.Lock()
	defer ws.emit.lock.Unlock()

	ws.emit.once[id] = append(ws.emit.once[id], fn)

}

// WHEN SYSTEM ISN'T BEEING USED, MAYBE FOR FUTURE IN-DATABASE EVENTS AND/OR REAL TIME stuffs.

func (ws *WS) when(id any, fn func(error, any)) {

	// pauses traffic in others threads, so we can add the new listener without conflicts
	ws.emit.lock.Lock()
	defer ws.emit.lock.Unlock()

	ws.emit.when[id] = append(ws.emit.when[id], fn)

}

func (ws *WS) done(id any, err error, res any) {

	// pauses traffic in others threads, so we can modify listeners without conflicts
	ws.emit.lock.Lock()
	defer ws.emit.lock.Unlock()

	// if our events map exist
	if ws.emit.when != nil {

		// if there's some listener aiming to this id response
		if when, ok := ws.emit.when[id]; ok {

			// dispatch the event, starting from the end, so we prioritize the new ones
			for i := len(when) - 1; i >= 0; i-- {

				// invoke callback
				when[i](err, res)

			}
		}
	}

	// if our events map exist
	if ws.emit.once != nil {

		// if theres some listener aiming to this id response
		if once, ok := ws.emit.once[id]; ok {

			// dispatch the event, starting from the end, so we prioritize the new ones
			for i := len(once) - 1; i >= 0; i-- {

				// invoke callback
				once[i](err, res)

				// erase this listener
				once[i] = nil

			}

			// remove all listeners
			ws.emit.once[id] = once[0:]
		}
	}

}

func (ws *WS) read() (response *RPCRawResponse, err error) {
	_, r, err := ws.ws.NextReader()
	if err != nil {
		return nil, err
	}

	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return CreateRPCRawResponse(raw), nil
}

func (ws *WS) write(v any) (err error) {
	w, err := ws.ws.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(w)
	// the default HTML escaping messes with select arrows
	enc.SetEscapeHTML(false)
	err = enc.Encode(v)
	if err != nil {
		return err
	}

	return w.Close()
}

func (ws *WS) initialise(ctx context.Context) {
	send := make(chan *RPCRequest)
	recv := make(chan *RPCRawResponse)
	ctx, cancel := context.WithCancel(ctx)

	// RECEIVER LOOP
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				res, err := ws.read()

				if err != nil {
					ws.Close()
					cancel()
					return
				}

				recv <- res // redirect response to: MAIN LOOP
			}
		}
	}()

	// SENDER LOOP
	go func() {
		for {
			select {
			case <-ctx.Done():
				return // stops: THIS LOOP
			case res := <-send:
				err := ws.write(res) // marshal and send

				if err != nil {
					ws.Close()
					cancel()
					return // stops: THIS LOOP
				}
			}
		}
	}()

	// MAIN LOOP
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case res := <-ws.recv:
				if res.HasInternalError() {
					log.Println("There was an error whilst decoding the RPC response: ", res.internalProcessingError)
				}

				ws.done(res.Id(), res.Error(), res)
			}
		}
	}()

	ws.send = send
	ws.recv = recv
}
