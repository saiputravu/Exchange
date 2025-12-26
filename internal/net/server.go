package net

import (
	"context"
	"errors"
	. "fenrir/internal/common"
	"fenrir/internal/utils"
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
	ErrClientDoesNotExist = errors.New("client does not exist")
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
	address            string
	port               int
	pool               utils.WorkerPool
	cancel             context.CancelFunc
	clientSessions     map[string]ClientSession
	clientSessionsLock sync.Mutex
	clientMessages     chan (ClientMessage)
}

func New(address string, port int) *Server {
	return &Server{
		address:        address,
		port:           port,
		pool:           utils.NewWorkerPool(defaultNWorkers),
		clientSessions: make(map[string]ClientSession),
		clientMessages: make(chan ClientMessage, 1),
	}
}

func (s *Server) Shutdown() {
	log.Info().Msg("server shutting down")
	s.cancel()
}

func (s *Server) Run(ctx context.Context) {
	defer s.Shutdown()

	// Setup a cancel on the context for future shutdown.
	ctx, s.cancel = context.WithCancel(ctx)
	t, ctx := tomb.WithContext(ctx)

	// Start a tcp listener.
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", fmt.Sprintf("%s:%d", s.address, s.port))
	if err != nil {
		log.Error().Err(err).Msg("unable to start listener")
		return
	}
	defer func() {
		if err := listener.Close(); err != nil {
			log.Error().Err(err).Msg("unable to close listener")
		}
	}()

	// Start the worker pool.
	t.Go(func() error {
		s.pool.Setup(t, s.handleConnection)
		return nil
	})

	// Start the session handler.
	t.Go(func() error {
		return s.sessionHandler(t)
	})

	log.Info().Msg("server running")

	// Start accepting connections.
	for {
		select {
		case <-ctx.Done():
			return
		default:
			log.Info().Msg("listening for new client connections")
			conn, err := listener.Accept()
			if err != nil {
				log.Error().Err(err).Msg("error accepting client")
				continue
			}

			log.Info().
				Str("address", conn.LocalAddr().String()).
				Msg("new client added")
			// Add the client to client sessions we are tracking.
			// We expect to potentially maintain a long TCP session.
			s.addClientSession(conn)

			// Pass over the connection to be read from.
			s.pool.AddTask(conn)
		}
	}
}

// FIXME: implement.
func (s *Server) Report(clientAddress string, trade Trade) error {
	s.clientSessionsLock.Lock()
	defer s.clientSessionsLock.Unlock()

	client, ok := s.clientSessions[clientAddress]
	if !ok {
		return ErrClientDoesNotExist
	}

	// FIXME: Fix the encoding.
	_, err := client.conn.Write([]byte(trade.String()))
	if err != nil {
		delete(s.clientSessions, clientAddress)
		return fmt.Errorf("unable to send report: %w", err)
	}

	return nil
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
			log.Info().
				Int("message type", int(message.message.typeOf)).
				Str("msg", message.message.field).
				Msg("new message")
		}
	}
}

// handleConnection is a short-lived worker method which reads the next message off the
// connection, parses and passes it forward to sessionHandler to handle it. If the connection
// dies, the client ssession is cleaned up. This method does not lock any client session
// directly and gives up early if the connection is terminated. Therefore this method is
// thread safe on map accesses.
// Note, any error returned from here is fatal.
func (s *Server) handleConnection(t *tomb.Tomb, task any) error {
	conn, ok := task.(net.Conn)
	if !ok {
		return ErrImproperConversion
	}

	defer func() {
		if err := conn.Close(); err != nil {
			log.Error().Str("address", conn.LocalAddr().String()).Err(err)
		}
	}()

	// Set max read timeout.
	err := conn.SetDeadline(time.Now().Add(defaultConnTimeout))
	if err != nil {
		log.Error().
			Str("address", conn.LocalAddr().Network()).
			Err(err).
			Msg("failed setting deadline for connection")
		return nil
	}

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
			return nil
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
		s.pool.AddTask(conn)
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
