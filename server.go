package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Server represents a TCP server that accepts incoming connections and handles client authentication.
//
// Fields:
// - p uint16: the listening port of the server.
// - rI RangeInclusive: the range of TCP ports that can be forwarded.
// - a *Authenticator: an optional secret used to authenticate clients.
// - c sync.Map: a concurrent map of IDs to incoming connections.
// - pt map[uint16]bool: a map to keep track of occupied ports.
// - ptMu sync.Mutex: a mutex to protect access to the ports map.
// - d *TcpClientRepository: a repository for accessing TCP client data.
type Server struct {
	p    uint16
	rI   RangeInclusive  // Range of TCP ports that can be forwarded.
	a    *Authenticator  // Optional secret used to authenticate clients.
	c    sync.Map        // Concurrent map of IDs to incoming connections.
	pt   map[uint16]bool // Map to keep track of occupied ports.
	ptMu sync.Mutex      // Mutex to protect access to ports map.
	d    *TcpClientRepository
}

// NewServer creates a new server instance.
// It takes a port number (p), a RangeInclusive object (rI),
// a TcpClientRepository object (d), and a pointer to a string (s) as parameters.
// Optionally, it creates a new Authenticator object using the string pointer.
// It returns a pointer to a Server object.
func NewServer(p uint16, rI RangeInclusive, d *TcpClientRepository, s *string) *Server {
	var a *Authenticator
	if s != nil {
		a = NewAuthenticator(*s, d, &rI)
	}
	return &Server{
		p:  p,
		rI: rI,
		a:  a,
		c:  sync.Map{},
		pt: make(map[uint16]bool),
		d:  d,
	}
}

// StartServer starts the server by listening for incoming TCP connections on the specified port.
// It creates a listener and accepts connections asynchronously, handling each connection
// by invoking the `handleConnection` method.
// The method blocks indefinitely and returns an error only when there is a failure in starting
// the listener or accepting a connection.
//
// Error handling:
// - If there is an error starting the listener, it returns an error with the root cause.
// - If there is an error accepting a connection, it returns an error with the root cause.
//
// This method is intended to be used in a goroutine.
func (s *Server) StartServer() error {
	a := fmt.Sprintf("0.0.0.0:%d", s.p)
	l, e := net.Listen("tcp", a)
	if e != nil {
		return fmt.Errorf("error starting listener: %w", e)
	}
	log.Printf("Server listening on %s\n", a)

	// Create a channel to listen for an interrupt or termination signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	shutdownCh := make(chan struct{})

	// Goroutine to handle shutdown signal.
	go func() {
		<-sigCh
		log.Println("Shutdown signal received, closing server...")
		close(shutdownCh)
		l.Close() // Close the listener to stop accepting new connections
	}()

	for {
		select {
		case <-shutdownCh:
			log.Println("Server gracefully shut down")
			return nil
		default:
			c, e := l.Accept()
			if e != nil {
				select {
				case <-shutdownCh:
					log.Println("Listener closed, shutting down connection handler")
					return nil
				default:
					return fmt.Errorf("error accepting connection: %w", e)
				}
			}
			go func() {
				log.Println("Incoming connection")
				if e := s.handleConnection(c); e != nil {
					log.Printf("Connection exited with error: %v\n", e)
				} else {
					log.Println("Connection exited")
				}
			}()
		}
	}
}

// handleConnection handles a single client connection. It performs the server handshake,
// receives and processes messages from the client, and forwards incoming connections
// to the appropriate client connection. If any errors occur during these processes,
// they are logged and returned as an error.
func (s *Server) handleConnection(c net.Conn) error {
	defer c.Close()
	st := NewCodec(c)
	if s.a != nil {
		if e := s.a.PerformServerHandshake(st); e != nil {
			log.Printf("Server handshake failed: %v\n", e)
			if e := st.Send(ServerMessage{Type: "Error", Error: e.Error()}); e != nil {
				return fmt.Errorf("error sending handshake error: %w", e)
			}
			return nil
		}
	}

	var m ClientMessage
	ctx, cn := context.WithTimeout(context.Background(), NetworkTimeout)
	defer cn()
	if e := st.Recv(ctx, &m); e != nil {
		return fmt.Errorf("error receiving message: %w", e)
	}

	switch m.Type {
	case MtAuthenticate:
		log.Println("Unexpected authenticate message")
		return nil
	case MtHello:
		l, p, e := s.createListenerForPort(m.Port)
		if e != nil {
			if se := st.Send(ServerMessage{Type: "Error", Error: e.Error()}); se != nil {
				return fmt.Errorf("error sending error message: %w", se)
			}
			return fmt.Errorf("error creating listener: %w", e)
		}
		log.Printf("New client on port %d\n", p)
		defer func() {
			l.Close()
			s.releasePort(p)
		}()

		if e := st.Send(ServerMessage{Type: "Hello", Port: p}); e != nil {
			return fmt.Errorf("error sending hello message: %w", e)
		}

		hb := time.NewTicker(HeartbeatInterval)
		defer hb.Stop()

		go func() {
			for {
				select {
				case <-hb.C:
					if e := st.Send(ServerMessage{Type: "Heartbeat"}); e != nil {
						l.Close()
						s.releasePort(p)
						log.Printf("Connection to port %d lost, port released\n", p)
						return
					}
					c.SetDeadline(time.Now().Add(ConnectionTimeout))
				}
			}
		}()

		for {
			cc, e := l.Accept()
			if e != nil {
				var ne net.Error
				if errors.As(e, &ne) && ne.Timeout() {
					continue
				}
				return fmt.Errorf("error accepting client connection: %w", e)
			}
			log.Printf("New connection on port %d\n", p)

			id := uuid.New()
			s.c.Store(id, cc)
			go func(id uuid.UUID) {
				time.Sleep(ConnectionTimeout)
				if _, ok := s.c.LoadAndDelete(id); ok {
					log.Printf("Removed stale connection %s\n", id)
				}
			}(id)
			if e := st.Send(ServerMessage{Type: "Connection", Connection: id}); e != nil {
				return fmt.Errorf("error sending connection message: %w", e)
			}
		}
	case MtAccept:
		log.Printf("Forwarding connection %s\n", m.Accept)
		if rc, ok := s.c.LoadAndDelete(m.Accept); ok {
			rc := rc.(net.Conn)

			// Function to handle copying data between connections
			ioCopy := func(dst io.Writer, src io.Reader) error {
				_, e := io.Copy(dst, src)
				return e
			}

			// Manage the copying of data concurrently using an error group.
			eg := new(errgroup.Group)
			eg.Go(func() error { return ioCopy(c, rc) })
			eg.Go(func() error { return ioCopy(rc, c) })

			if e := eg.Wait(); e != nil {
				return fmt.Errorf("error during data forwarding: %w", e)
			}
		} else {
			log.Printf("Missing connection %s\n", m.Accept)
		}
	}

	return nil
}

// createListenerForPort creates a listener for a given port.
// If the provided port is greater than zero, it checks if the port
// is within the allowed range. If it is not, it returns an error.
// Otherwise, it tries to bind the port. If binding is successful,
// it marks the port as occupied and returns the listener and port.
// If no port is provided or if binding fails, it generates a random
// port within the allowed range and repeats the above steps for a
// maximum of 150 attempts. If all attempts fail, it returns an error.
// The function uses the tryBindingPort function to bind the port.
// The port occupation status is managed by the Server's pt map,
// protected by a mutex.
//
// Parameters:
// - p: The port number to create a listener for.
//
// Returns:
//   - net.Listener: The listener for the port.
//   - uint16: The port number that was used.
//   - error: Any error that occurred during listener creation.
//     Possible errors include ErrMsgPortNotInRange, ErrMsgBindingPort,
//     ErrMsgPortInUse, and ErrMsgPermissionDenied.
func (s *Server) createListenerForPort(p uint16) (net.Listener, uint16, error) {
	if p > 0 {
		if !s.rI.Contains(p) {
			return nil, 0, ErrMsgPortNotInRange
		}
		l, e := tryBindingPort(p)
		if e == nil {
			s.occupyPort(p)
		}
		return l, p, e
	}
	for i := 0; i < 150; i++ {
		p := s.rI.RandomPort()
		if !s.isPortAlreadyOccupied(p) {
			l, e := tryBindingPort(p)
			if e == nil {
				s.occupyPort(p)
				return l, p, nil
			}
		}
	}
	return nil, 0, ErrMsgAvailablePort
}

// tryBindingPort attempts to bind a TCP listener to a specific port.
// If the binding is successful, it returns the listener and nil error.
// If the port is already in use, it returns nil listener and ErrMsgPortInUse error.
// If there is a permission denied error, it returns nil listener and ErrMsgPermissionDenied error.
// For any other error, it returns nil listener and ErrMsgBindingPort error.
func tryBindingPort(p uint16) (net.Listener, error) {
	a := fmt.Sprintf("0.0.0.0:%d", p)
	l, e := net.Listen("tcp", a)
	if e != nil {
		var oe *net.OpError
		if errors.As(e, &oe) {
			var ae *net.AddrError
			var se *os.SyscallError
			switch {
			case errors.As(oe.Err, &ae):
				return nil, ErrMsgPortInUse
			case errors.As(oe.Err, &se):
				if errors.Is(oe.Err, syscall.EADDRINUSE) {
					return nil, ErrMsgPortInUse
				}
				return nil, ErrMsgPermissionDenied
			}
		}
		return nil, ErrMsgBindingPort
	}
	return l, nil
}

// occupyPort acquires ownership of a port by updating the ports map of
// the Server object. The method locks the port mutex, adds the specified
// port to the map with a boolean value of true, and then releases the
// lock. This method is used internally to mark a port as occupied when
// creating a listener for a port or accepting a client connection.
func (s *Server) occupyPort(p uint16) {
	s.ptMu.Lock()
	defer s.ptMu.Unlock()
	s.pt[p] = true
}

// releasePort releases a port in the server's ports map.
func (s *Server) releasePort(p uint16) {
	s.ptMu.Lock()
	defer s.ptMu.Unlock()
	delete(s.pt, p)
}

// isPortAlreadyOccupied checks if a given port is already occupied by another connection.
// It locks the port mutex, checks if the given port is present in the port map, and returns
// the result. It is used internally in the Server struct to determine if a port is available
// for a new connection.
func (s *Server) isPortAlreadyOccupied(p uint16) bool {
	s.ptMu.Lock()
	defer s.ptMu.Unlock()
	return s.pt[p]
}
