package websocket

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

type WS struct {
	ws   *websocket.Conn     // websocket connection
	quit chan error          // stops: MAIN LOOP
	send chan<- *RPCRequest  // sender channel
	recv <-chan *RPCResponse // receive channel
	emit struct {
		lock sync.Mutex                                 // pause threads to avoid conflicts
		once map[interface{}][]func(error, interface{}) // once listeners
		when map[interface{}][]func(error, interface{}) // when listeners
	}
}

func NewWebsocket(url string) (*WS, error) {
	dialer := websocket.DefaultDialer
	dialer.EnableCompression = true

	// stablish connection
	so, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	ws := &WS{ws: so}
	// setup loops and channels
	ws.initialize()

	return ws, nil
}

// --------------------------------------------------
// Public methods
// --------------------------------------------------

func (ws *WS) Close() error {
	msg := websocket.FormatCloseMessage(1000, "") //nolint:gomnd
	return ws.ws.WriteMessage(websocket.CloseMessage, msg)
}

func (ws *WS) Send(id, method string, params []interface{}) {
	go func() {
		ws.send <- &RPCRequest{
			ID:     id,
			Method: method,
			Params: params,
		}
	}()
}

// Subscribe to once()
func (ws *WS) Once(id, method string) (<-chan interface{}, <-chan error) { //nolint:gocritic
	err := make(chan error)
	res := make(chan interface{})

	ws.once(id, func(e error, r interface{}) {
		switch {
		case e != nil:
			err <- e
			close(err)
			close(res)
		case e == nil:
			res <- r
			close(err)
			close(res)
		}
	})

	return res, err
}

// Subscribe to when()
func (ws *WS) When(id, method string) (<-chan interface{}, <-chan error) { //nolint:gocritic
	err := make(chan error)
	res := make(chan interface{})

	ws.when(id, func(e error, r interface{}) {
		switch {
		case e != nil:
			err <- e
		case e == nil:
			res <- r
		}
	})

	return res, err
}

// --------------------------------------------------
// Private methods
// --------------------------------------------------

func (ws *WS) once(id interface{}, fn func(error, interface{})) {
	// pauses traffic in others threads, so we can add the new listener without conflicts
	ws.emit.lock.Lock()
	defer ws.emit.lock.Unlock()

	// if its our first listener, we need to setup the map
	if ws.emit.once == nil {
		ws.emit.once = make(map[interface{}][]func(error, interface{}))
	}

	ws.emit.once[id] = append(ws.emit.once[id], fn)
}

// WHEN SYSTEM ISN'T BEING USED, MAYBE FOR FUTURE IN-DATABASE EVENTS AND/OR REAL TIME stuffs.
func (ws *WS) when(id interface{}, fn func(error, interface{})) {
	// pauses traffic in others threads, so we can add the new listener without conflicts
	ws.emit.lock.Lock()
	defer ws.emit.lock.Unlock()

	// if its our first listener, we need to setup the map
	if ws.emit.when == nil {
		ws.emit.when = make(map[interface{}][]func(error, interface{}))
	}

	ws.emit.when[id] = append(ws.emit.when[id], fn)
}

func (ws *WS) done(id interface{}, err error, res interface{}) {
	// pauses traffic in others threads, so we can modify listeners without conflicts
	ws.emit.lock.Lock()
	defer ws.emit.lock.Unlock()

	// if our events map exist
	if ws.emit.when != nil {
		// if theres some listener aiming to this id response
		if _, ok := ws.emit.when[id]; ok {
			// dispatch the event, starting from the end, so we prioritize the new ones
			for i := len(ws.emit.when[id]) - 1; i >= 0; i-- {
				// invoke callback
				ws.emit.when[id][i](err, res)
			}
		}
	}

	// if our events map exist
	if ws.emit.once != nil {
		// if theres some listener aiming to this id response
		if _, ok := ws.emit.once[id]; ok {
			// dispatch the event, starting from the end, so we prioritize the new ones
			for i := len(ws.emit.once[id]) - 1; i >= 0; i-- {
				// invoke callback
				ws.emit.once[id][i](err, res)
				// erase this listener
				ws.emit.once[id][i] = nil
				// remove this listener from the list
				ws.emit.once[id] = ws.emit.once[id][:i]
			}
		}
	}
}

func (ws *WS) read(v interface{}) (err error) {
	_, r, err := ws.ws.NextReader()
	if err != nil {
		return err
	}

	return json.NewDecoder(r).Decode(v)
}

func (ws *WS) write(v interface{}) (err error) {
	w, err := ws.ws.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	err = json.NewEncoder(w).Encode(v)
	if err != nil {
		return err
	}

	return w.Close()
}

func (ws *WS) initialize() {
	send := make(chan *RPCRequest)
	recv := make(chan *RPCResponse)
	quit := make(chan error, 1) // stops: MAIN LOOP
	exit := make(chan int, 1)   // stops: RECEIVER LOOP, SENDER LOOP

	// RECEIVER LOOP
	go func() {
	loop:
		for {
			select {
			case <-exit:
				break loop // stops: THIS LOOP
			default:
				var res RPCResponse
				err := ws.read(&res) // wait and unmarshal UPCOMING response

				if err != nil {
					ws.Close()
					quit <- err // stops: MAIN LOOP
					exit <- 0   // stops: RECEIVER LOOP, SENDER LOOP
					break loop  // stops: THIS LOOP
				}

				recv <- &res // redirect response to: MAIN LOOP
			}
		}
	}()

	// SENDER LOOP
	go func() {
	loop:
		for {
			select {
			case <-exit:
				break loop // stops: THIS LOOP
			case res := <-send:

				err := ws.write(res) // marshal and send

				if err != nil {
					ws.Close()
					quit <- err // stops: MAIN LOOP
					exit <- 0   // stops: RECEIVER LOOP, SENDER LOOP
					break loop  // stops: THIS LOOP
				}
			}
		}
	}()

	// MAIN LOOP
	go func() {
	loop:
		for {
			select {
			case <-ws.quit:
				break loop
			case res := <-ws.recv:
				switch {
				case res.Error == nil:
					ws.done(res.ID, nil, res.Result)
				case res.Error != nil:
					ws.done(res.ID, res.Error, res.Result)
				}
			}
		}
	}()

	ws.send = send
	ws.recv = recv
	ws.quit = quit // stops: MAIN LOOP
}
