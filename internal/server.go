package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	tomb "gopkg.in/tomb.v2"
)

const (
	MAX_RECV_SIZE      = 4 * 1024
	defaultNWorkers    = 10
	defaultConnTimeout = time.Second
)

var (
	ErrImproperConversion = errors.New("improper type conversion")
)

// ClientSession contains relevant information pertaining to an individual
// connected TCP session.
type ClientSession struct {
	conn net.Conn
}

// ClientMessage links a message to the client sending it.
type ClientMessage struct {
	clientAddress string
	message       Message
}

type Server struct {
	listener           net.Listener
	pool               WorkerPool
	cancel             context.CancelFunc
	clientSessions     map[string]ClientSession
	clientSessionsLock sync.Mutex
	clientMessages     chan (ClientMessage)
}

func New(address string, port int) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", address, port))
	if err != nil {
		return nil, err
	}
	pool := NewWorkerPool(defaultNWorkers)
	return &Server{
		listener: listener,
		pool:     pool,
	}, nil
}

func (s *Server) Shutdown() {
	log.Info().Msg("server shutting down")
	if err := s.listener.Close(); err != nil {
		log.Error().Err(err).Msg("unable to close listener")
	}
	s.cancel()
}

func (s *Server) Run(ctx context.Context) {
	// Setup a cancel on the context for future shutdown.
	ctx, s.cancel = context.WithCancel(ctx)
	defer s.cancel()

	// Start the worker pool.
	t, ctx := tomb.WithContext(ctx)
	s.pool.Setup(t, s.handleConnection)

	// Start the session handler.
	t.Go(func() error {
		return s.sessionHandler(t)
	})

	// Start accepting connections.
	for {
		select {
		case <-ctx.Done():
			s.Shutdown()
			return
		default:
			log.Debug().Msg("listening for new client connections")
			conn, err := s.listener.Accept()
			if err != nil {
				log.Error().Err(err).Msg("error accepting client")
			}

			log.Debug().
				Str("address", conn.LocalAddr().String()).
				Msg("new client added")
			// Add the client to client sessions we are tracking.
			// We expect to potentially maintain a long TCP session.
			s.addClientSession(conn)

			// Pass over the connection to be read from.
			s.pool.tasks <- conn
		}
	}
}

// sessionHandler reads off incoming messages from clients and handles high-level
// session logic. Messages are received from the pool of workers.
func (s *Server) sessionHandler(t *tomb.Tomb) error {
	for {
		select {
		case <-t.Dying():
			return nil
		case message := <-s.clientMessages:
			// FIXME: implement this.
			log.Info().Any("message", message).Msg("new message")
		}
	}
}

// handleConnection is a short-lived worker method which reads
// the next message off the connection, parses and passes it
// forward to sessionHandler to handle it. If the connection
// dies, the client session is cleaned up.
func (s *Server) handleConnection(t *tomb.Tomb, task any) error {
	conn, ok := task.(net.Conn)
	if !ok {
		return ErrImproperConversion
	}

	// Set max read timeout.
	conn.SetDeadline(time.Now().Add(defaultConnTimeout))

	defer func() {
		if err := conn.Close(); err != nil {
			log.Error().Err(err)
		}
	}()

	buffer := make([]byte, MAX_RECV_SIZE)
	select {
	case <-t.Dying():
		return nil
	default:
		n, err := conn.Read(buffer)
		if err != nil {
			log.Error().
				Err(err).
				Str("address", conn.LocalAddr().String()).
				Msg("error reading from connection")

			// If a read from a client fails, it is likely that the client
			// has exited. Clean up the client session.
			// TODO: Should handle this properly and check for graceful EOF.
			s.deleteClientSession(conn.LocalAddr().String())
		}

		message, err := parseMessage(buffer[:n])
		if err != nil {
			log.Error().
				Err(err).
				Str("address", conn.LocalAddr().String()).
				Msg("error parsing message")
			s.deleteClientSession(conn.LocalAddr().String())
		}

		// Pass over to the message handling buffer and exit this worker.
		s.clientMessages <- ClientMessage{
			message:       message,
			clientAddress: conn.LocalAddr().String(),
		}

		// Push the client connection back to handle the next message.
		s.pool.tasks <- conn
	}
	return nil
}

// addClientSession is an atomic map add
func (s *Server) addClientSession(conn net.Conn) {
	s.clientSessionsLock.Lock()
	defer s.clientSessionsLock.Unlock()

	s.clientSessions[conn.LocalAddr().String()] = ClientSession{
		conn: conn,
	}
}

// deleteClientSession is an atomic map remove
func (s *Server) deleteClientSession(address string) {
	s.clientSessionsLock.Lock()
	defer s.clientSessionsLock.Unlock()

	delete(s.clientSessions, address)
}
