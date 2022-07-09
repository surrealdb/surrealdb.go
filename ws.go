package surrealdb

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"sync"
)

type WS struct {
	ws   *websocket.Conn
	quit chan error
	send chan<- *RPCRequest
	recv <-chan *RPCResponse
	emit struct {
		lock sync.Mutex
		once map[any][]func(error, any)
		when map[any][]func(error, any)
	}
}

func NewWebsocket(url string) (*WS, error) {

	dialer := websocket.DefaultDialer

	dialer.EnableCompression = true

	so, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	ws := &WS{ws: so}

	ws.initialise()

	return ws, nil

}

// --------------------------------------------------
// Public methods
// --------------------------------------------------

func (self *WS) Close() error {

	msg := websocket.FormatCloseMessage(1000, "")

	err := self.ws.WriteMessage(websocket.CloseMessage, msg)

	return err

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

	self.emit.lock.Lock()
	defer self.emit.lock.Unlock()

	if self.emit.once == nil {
		self.emit.once = make(map[any][]func(error, any))
	}

	self.emit.once[id] = append(self.emit.once[id], fn)

}

func (self *WS) when(id any, fn func(error, any)) {

	self.emit.lock.Lock()
	defer self.emit.lock.Unlock()

	if self.emit.when == nil {
		self.emit.when = make(map[any][]func(error, any))
	}

	self.emit.when[id] = append(self.emit.when[id], fn)

}

func (self *WS) done(id any, err error, res any) {

	self.emit.lock.Lock()
	defer self.emit.lock.Unlock()

	if self.emit.when != nil {
		if _, ok := self.emit.when[id]; ok {
			for i := len(self.emit.when[id]) - 1; i >= 0; i-- {
				self.emit.when[id][i](err, res)
			}
		}
	}

	if self.emit.once != nil {
		if _, ok := self.emit.once[id]; ok {
			for i := len(self.emit.once[id]) - 1; i >= 0; i-- {
				self.emit.once[id][i](err, res)
				self.emit.once[id][i] = nil
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
	quit := make(chan error, 1)
	exit := make(chan int, 1)

	go func() {
	loop:
		for {
			select {
			case <-exit:
				break loop
			default:

				var res RPCResponse

				err := self.read(&res)

				if err != nil {
					self.Close()
					quit <- err
					exit <- 0
					break loop
				}

				recv <- &res

			}
		}
	}()

	go func() {
	loop:
		for {
			select {
			case <-exit:
				break loop
			case res := <-send:

				err := self.write(res)

				if err != nil {
					self.Close()
					quit <- err
					exit <- 0
					break loop
				}

			}
		}
	}()

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
	self.quit = quit

}
