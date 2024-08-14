package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/google/uuid"
	"net"
	"slices"
	"strconv"
	"strings"
)

// Authenticator represents an object responsible for handling client authentication and generating and validating answers.
//
// Fields:
// - k []byte: the secret key used for generating answers and validating answers.
// - pr *RangeInclusive: the range of ports used for finding free ports during authentication.
// - db *TcpClientRepository: the repository for accessing TCP client data.
//
// Methods:
// - GenerateAnswer(ch uuid.UUID) string: generates an answer for a challenge.
// - ValidateAnswer(ch uuid.UUID, ans string) bool: validates an answer for a challenge.
// - PerformServerHandshake(stream *Codec) error: performs the server handshake with the client.
// - handleClientAuth(stream *Codec, id string) error: handles the client authentication process.
// - findFreePort() (uint16, error): finds a free port within the specified range.
type Authenticator struct {
	k  []byte
	pr *RangeInclusive
	db *TcpClientRepository
}

// NewAuthenticator creates a new instance of the Authenticator struct and initializes it with the provided parameters.
// The secret parameter is used to generate the authentication key, which is a SHA-256 hash of the secret.
// The db parameter is a reference to a TcpClientRepository, which is used to interact with the client database.
// The pr parameter is a reference to a RangeInclusive struct, which represents the range of ports that can be used.
// The function returns a pointer to the newly created Authenticator instance.
func NewAuthenticator(secret string, db *TcpClientRepository, pr *RangeInclusive) *Authenticator {
	h := sha256.Sum256([]byte(secret))
	return &Authenticator{k: h[:], pr: pr, db: db}
}

// GenerateAnswer generates an answer using the HMAC-SHA256 algorithm.
// It takes a uuid.UUID as a challenge, appends it to the key provided during
// Authenticator initialization, and computes the HMAC-SHA256 hash. The result
// is then encoded to a hexadecimal string and returned.
func (a *Authenticator) GenerateAnswer(ch uuid.UUID) string {
	m := hmac.New(sha256.New, a.k)
	m.Write(ch[:])
	return hex.EncodeToString(m.Sum(nil))
}

// ValidateAnswer validates the answer provided by the client for a challenge.
// It decodes the answer from a hex-string to bytes and computes the HMAC of
// the challenge using the provided key. Then it checks if the computed HMAC
// is equal to the decoded answer. Returns true if the answer is valid, false
// otherwise.
func (a *Authenticator) ValidateAnswer(ch uuid.UUID, ans string) bool {
	b, err := hex.DecodeString(ans)
	if err != nil {
		return false
	}
	m := hmac.New(sha256.New, a.k)
	m.Write(ch[:])
	em := m.Sum(nil)
	return hmac.Equal(em, b)
}

// PerformServerHandshake performs the server-side handshake process with a client.
// It sends a challenge message to the client, receives the client's response,
// validates it, and handles the authentication process if the response is valid.
// If the handshake fails at any step, an error is returned.
func (a *Authenticator) PerformServerHandshake(stream *Codec) error {
	ch := uuid.New()
	if err := stream.Send(ServerMessage{Type: MtChallenge, Challenge: ch}); err != nil {
		return err
	}

	var msg ClientMessage
	ctx, cancel := context.WithTimeout(context.Background(), NetworkTimeout)
	defer cancel()

	if err := stream.Recv(ctx, &msg); err != nil {
		return err
	}

	if msg.Type == MtAuthenticate && a.ValidateAnswer(ch, msg.Authenticate) {
		if err := a.handleClientAuth(stream, msg.ClientId); err != nil {
			return err
		}
	} else {
		return ErrInvalidSecret
	}

	return nil
}

// `handleClientAuth` handles the authentication of a client. It validates the client ID, retrieves the client information from the database, generates a free port if necessary, stores the client information in the database, and sends the free port to the client through the stream.
func (a *Authenticator) handleClientAuth(stream *Codec, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidClientId
	}

	c, ex, err := a.db.GetByClientID(id)
	if err != nil {
		return err
	}

	var p uint16
	if !ex {
		bl, err := a.db.GetAllPorts()
		if err != nil {
			return err
		}
		p, err = a.findFreePort(bl)
		if err != nil {
			return err
		}
		err = a.db.Create(&TCPClient{ID: p, ClientID: id, Port: p})
		if err != nil {
			return err
		}
	} else {
		p = c.Port
	}

	return stream.Send(ServerMessage{Type: MtFreePort, Port: p})
}

// findFreePort finds a free port within the range of min and max ports specified in the Authenticator's RangeInclusive property.
// It tries to listen on each port in the range and returns the first port that is available.
// If no port is available, it returns an error indicating that no port was found in the specified range.
func (a *Authenticator) findFreePort(skip []uint16) (uint16, error) {
	for p := a.pr.min; p <= a.pr.max; p++ {
		if slices.Contains(skip, p) {
			continue
		}
		l, err := net.Listen("tcp", ":"+strconv.Itoa(int(p)))
		if err == nil {
			l.Close()
			return p, nil
		}
	}
	return 0, fmt.Errorf("no available port found in the range %d-%d", a.pr.min, a.pr.max)
}
