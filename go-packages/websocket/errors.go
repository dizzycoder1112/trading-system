package websocket

import "errors"

var (
	ErrNotConnected = errors.New("websocket not connected")
	ErrAlreadyConnected = errors.New("websocket already connected")
)
