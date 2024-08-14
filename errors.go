package main

import "errors"

var (
	ErrInvalidClientId     = errors.New("invalid client id")
	ErrInvalidSecret       = errors.New("invalid secret")
	ErrMsgPortNotInRange   = errors.New("client port number not in allowed range")
	ErrMsgAvailablePort    = errors.New("failed to find an available port")
	ErrMsgBindingPort      = errors.New("failed to bind to port")
	ErrMsgPermissionDenied = errors.New("permission denied")
	ErrMsgPortInUse        = errors.New("port already in use")
)
