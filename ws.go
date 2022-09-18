package surrealdb

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

type WS struct {
	ws   *websocket.Conn // websocket connection
	quit chan error // stops: MAIN LOOP
	send chan<- *RPCRequest // sender channel
	recv <-chan *RPCResponse // receive channel
	emit struct {
		lock sync.Mutex // pause threads to avoid conflicts
		once map[any][]func(error, any) // once listeners
		when map[any][]func(error, any) // when listeners
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
	ws.initialise()

	return ws, nil

}

// --------------------------------------------------
// Public methods
// --------------------------------------------------

func (self *WS) Close() error {

	msg := websocket.FormatCloseMessage(1000, "")
	return self.ws.WriteMessage(websocket.CloseMessage, msg)

}

func (self *WS) Send(id string, method string, params []any) {

	go func() {
		self.send <- &RPCRequest{
			ID:     id,
			Method: method,
			Params: params,
		}
	}()

}

// Subscribe to once()
func (self *WS) Once(id, method string) (<-chan any, <-chan error) {

	err := make(chan error)
	res := make(chan any)

	self.once(id, func(e error, r any) {
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
func (self *WS) When(id, method string) (<-chan any, <-chan error) {

	err := make(chan error)
	res := make(chan any)

	self.when(id, func(e error, r any) {
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

func (self *WS) once(id any, fn func(error, any)) {

	// pauses traffic in others threads, so we can add the new listener without conflicts
	self.emit.lock.Lock()
	defer self.emit.lock.Unlock()

	// if its our first listener, we need to setup the map
	if self.emit.once == nil {
		self.emit.once = make(map[any][]func(error, any))
	}

	self.emit.once[id] = append(self.emit.once[id], fn)

}

// WHEN SYSTEM ISN'T BEEING USED, MAYBE FOR FUTURE IN-DATABASE EVENTS AND/OR REAL TIME stuffs.

func (self *WS) when(id any, fn func(error, any)) {

	// pauses traffic in others threads, so we can add the new listener without conflicts
	self.emit.lock.Lock()
	defer self.emit.lock.Unlock()

	// if its our first listener, we need to setup the map
	if self.emit.when == nil {
		self.emit.when = make(map[any][]func(error, any))
	}

	self.emit.when[id] = append(self.emit.when[id], fn)

}

func (self *WS) done(id any, err error, res any) {

	// pauses traffic in others threads, so we can modify listeners without conflicts
	self.emit.lock.Lock()
	defer self.emit.lock.Unlock()

	// if our events map exist
	if self.emit.when != nil {

		// if theres some listener aiming to this id response
		if _, ok := self.emit.when[id]; ok {

			// dispatch the event, starting from the end, so we prioritize the new ones
			for i := len(self.emit.when[id]) - 1; i >= 0; i-- {

				// invoke callback
				self.emit.when[id][i](err, res)

			}
		}
	}

	// if our events map exist
	if self.emit.once != nil {

		// if theres some listener aiming to this id response
		if _, ok := self.emit.once[id]; ok {

			// dispatch the event, starting from the end, so we prioritize the new ones
			for i := len(self.emit.once[id]) - 1; i >= 0; i-- {

				// invoke callback
				self.emit.once[id][i](err, res)

				// erase this listener
				self.emit.once[id][i] = nil

				// remove this listener from the list
				self.emit.once[id] = self.emit.once[id][:i]
			}
		}
	}

}

func (self *WS) read(v any) (err error) {

	_, r, err := self.ws.NextReader()
	if err != nil {
		return err
	}

	return json.NewDecoder(r).Decode(v)

}

func (self *WS) write(v any) (err error) {

	w, err := self.ws.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	err = json.NewEncoder(w).Encode(v)
	if err != nil {
		return err
	}

	return w.Close()

}

func (self *WS) initialise() {
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
				err := self.read(&res) // wait and unmarshal UPCOMING response

				if err != nil {
					self.Close()
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

				err := self.write(res) // marshal and send

				if err != nil {
					self.Close()
					quit <- err // stops: MAIN LOOP
					exit <- 0   // stops: RECEIVER LOOP, SENDER LOOP
					break loop  // stops: THIS LOOP
				}

			}
		}
	}()

	// MAIN LOOP

	go func() {
		for {
			select {
			case <-self.quit:
				break
			case res := <-self.recv:
				switch {
				case res.Error == nil:
					self.done(res.ID, nil, res.Result)
				case res.Error != nil:
					self.done(res.ID, res.Error, res.Result)
				}
			}
		}
	}()

	self.send = send
	self.recv = recv
	self.quit = quit // stops: MAIN LOOP
}
