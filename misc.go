package main

import (
	"github.com/google/uuid"
	"time"
)

const (
	// NetworkTimeout specifies the timeout for network connections and initial protocol messages.
	NetworkTimeout    = 10 * time.Second
	HeartbeatInterval = 2 * time.Second
	ConnectionTimeout = 30 * time.Second
	DbName            = ".jerusalem.db"
	DbDriver          = "sqlite3"
	DefaultSecret     = "2y6sUp8cBSfNDk7Jq5uLm0xHAIOb9ZGqE4hR1WVXtCwKjP3dYzvTn2QiFXe8rMb6"
)

const (
	MtChallenge    = "Challenge"
	MtAuthenticate = "Authenticate"
	MtFreePort     = "FreePort"
	MtAccept       = "Accept"
	MtHello        = "Hello"
)

type ClientMessage struct {
	Type         string    `json:"type"`
	Authenticate string    `json:"authenticate,omitempty"`
	Port         uint16    `json:"port,omitempty"`
	Accept       uuid.UUID `json:"accept,omitempty"`
	ClientId     string    `json:"clientId,omitempty"`
}

type ServerMessage struct {
	Type       string    `json:"type"`
	Challenge  uuid.UUID `json:"challenge,omitempty"`
	Port       uint16    `json:"hello,omitempty"`
	Heartbeat  bool      `json:"heartbeat,omitempty"`
	Connection uuid.UUID `json:"connection,omitempty"`
	Error      string    `json:"error,omitempty"`
}
