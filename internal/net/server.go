package net

import (
	"context"
	"errors"
	. "fenrir/internal/common"
	"fenrir/internal/utils"
	"fmt"
	"net"
	"sync"

	"github.com/rs/zerolog/log"
	tomb "gopkg.in/tomb.v2"
)

const (
	MAX_RECV_SIZE   = 4 * 1024
	defaultNWorkers = 10
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

// TODO: Maybe move this to common/
// Engine is interface that provides access to order handling.
type Engine interface {
	PlaceOrder(assetType AssetType, order Order) error
	CancelOrder(assetType AssetType, uuid string) error
	LogBook()
}

type Server struct {
	address            string
	port               int
	engine             Engine
	pool               utils.WorkerPool
	cancel             context.CancelFunc
	clientSessions     map[string]ClientSession
	clientSessionsLock sync.Mutex
	clientMessages     chan (ClientMessage)
}

func New(address string, port int, engine Engine) *Server {
	return &Server{
		address:        address,
		port:           port,
		engine:         engine,
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
				Str("address", conn.RemoteAddr().String()).
				Msg("new client added")

			// Add the client to client sessions we are tracking.
			// We expect to potentially maintain a long TCP session.
			s.addClientSession(conn)

			// Pass over the connection to be read from.
			s.pool.AddTask(conn)
		}
	}
}

func (s *Server) ReportTrade(trade Trade, err error) error {
	s.clientSessionsLock.Lock()
	defer s.clientSessionsLock.Unlock()

	partyReport, counterPartyReport, err := generateWireTradeReports(trade, err)
	if err != nil {
		return err
	}

	party, partyOk := s.clientSessions[trade.Party.Owner]
	counterParty, counterPartyOk := s.clientSessions[trade.CounterParty.Owner]
	log.Info().Str("party", trade.Party.Owner).Str("counter", trade.CounterParty.Owner).Msg("reporttrade")
	if !partyOk || !counterPartyOk {
		return fmt.Errorf("client does not exist: party [%v], counter [%v]", party, counterParty)
	}

	_, err = party.conn.Write(partyReport)
	if err != nil {
		s.deleteClientSessionLockFree(party.conn.RemoteAddr().String())
		return fmt.Errorf("unable to send report: %w", err)
	}

	_, err = counterParty.conn.Write(counterPartyReport)
	if err != nil {
		s.deleteClientSessionLockFree(counterParty.conn.RemoteAddr().String())
		return fmt.Errorf("unable to send report: %w", err)
	}
	return nil
}

func (s *Server) ReportOrderPlaced(clientAddress string, ord Order) error {
	report, err := generateWireOrderPlacedReport(ord)
	if err != nil {
		return err
	}

	s.clientSessionsLock.Lock()
	defer s.clientSessionsLock.Unlock()

	client, ok := s.clientSessions[clientAddress]
	if !ok {
		return ErrClientDoesNotExist
	}

	_, err = client.conn.Write(report)
	if err != nil {
		s.deleteClientSessionLockFree(clientAddress)
		return fmt.Errorf("unable to send report: %w", err)
	}
	return nil
}

func (s *Server) ReportError(clientAddress string, err error) error {
	report, err := generateWireErrorReports(err)
	if err != nil {
		return err
	}

	s.clientSessionsLock.Lock()
	defer s.clientSessionsLock.Unlock()

	client, ok := s.clientSessions[clientAddress]
	if !ok {
		return ErrClientDoesNotExist
	}

	_, err = client.conn.Write(report)
	if err != nil {
		s.deleteClientSessionLockFree(clientAddress)
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
			if err := s.handleMessage(t, message); err != nil {
				log.Error().
					Err(err).
					Str("clientAddress", message.clientAddress).
					Msg("error handling message")
				// Log the error back to the client
				s.ReportError(message.clientAddress, err)
			}
		}
	}
}

func (s *Server) handleMessage(t *tomb.Tomb, message ClientMessage) error {
	switch message.message.GetType() {
	case NewOrder:
		order, ok := message.message.(NewOrderMessage)
		if !ok {
			return ErrInvalidMessageType
		}
		ord, err := order.Order(message.clientAddress)
		if err != nil {
			return err
		}
		err = s.engine.PlaceOrder(order.AssetType, ord)
		if err != nil {
			s.ReportError(message.clientAddress, err)
			log.Error().
				Err(err).
				Str("clientAddress", message.clientAddress).
				Msg("error while placing order")
		}

		// Report back.
		t.Go(func() error {
			if err := s.ReportOrderPlaced(message.clientAddress, ord); err != nil {
				s.ReportError(message.clientAddress, err)
				log.Error().
					Err(err).
					Str("clientAddress", message.clientAddress).
					Msg("error while generating order")
			}
			return nil
		})
	case CancelOrder:
		// TODO: Implement
		order, ok := message.message.(CancelOrderMessage)
		if !ok {
			return ErrInvalidMessageType
		}
		err := s.engine.CancelOrder(order.AssetType, order.OrderUUID)
		if err != nil {
			s.ReportError(message.clientAddress, err)
			log.Error().
				Err(err).
				Str("clientAddress", message.clientAddress).
				Str("uuid", order.OrderUUID).
				Msg("error while cancelling order")
		}
	case LogBook:
		s.engine.LogBook()
	default:
		log.Error().
			Int("messageType", int(message.message.GetType())).
			Any("message", message).
			Msg("invalid message type")
		return ErrInvalidMessageType
	}
	return nil
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

	buffer := make([]byte, MAX_RECV_SIZE)
	select {
	case <-t.Dying():
		return nil
	default:
		n, err := conn.Read(buffer)
		if err != nil {
			// TODO: Think about heartbeating but I cba.
			log.Error().
				Err(err).
				Str("address", conn.RemoteAddr().String()).
				Msg("error reading from connection")
			s.deleteClientSession(conn.RemoteAddr().String())
			return nil
		}

		message, err := parseMessage(buffer[:n])
		if err != nil {
			log.Error().
				Err(err).
				Str("address", conn.RemoteAddr().String()).
				Msg("error parsing message")
			s.deleteClientSession(conn.RemoteAddr().String())
			return nil
		}

		// Pass over to the message handling buffer and exit this worker.
		s.clientMessages <- ClientMessage{
			message:       message,
			clientAddress: conn.RemoteAddr().String(),
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

	s.clientSessions[conn.RemoteAddr().String()] = ClientSession{
		conn: conn,
	}
}

// deleteClientSession is an atomic map remove
func (s *Server) deleteClientSession(address string) {
	s.clientSessionsLock.Lock()
	defer s.clientSessionsLock.Unlock()

	if client, ok := s.clientSessions[address]; ok {
		// Cleanup the connection object.
		if err := client.conn.Close(); err != nil {
			log.Error().
				Err(err).
				Str("clientAddress", address).
				Msg("unable to close client connection")
		}
		delete(s.clientSessions, address)
	}
}

// deleteClientSessionLockFree is intended to prevent renetrancy on locks.
func (s *Server) deleteClientSessionLockFree(address string) {
	if client, ok := s.clientSessions[address]; ok {
		// Cleanup the connection object.
		if err := client.conn.Close(); err != nil {
			log.Error().
				Err(err).
				Str("clientAddress", address).
				Msg("unable to close client connection")
		}
		delete(s.clientSessions, address)
	}
}
