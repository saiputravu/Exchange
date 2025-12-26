package net

import (
	"encoding/binary"
	"errors"
	. "fenrir/internal/common"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidMessageType = errors.New("invalid message type")
	ErrMessageTooShort    = errors.New("message too short for specified username length")
	ErrInvalidUUID        = errors.New("invalid uuid")
)

type MessageType int

const (
	Heartbeat MessageType = iota
	NewOrder
	CancelOrder
)

type ReportMessageType int

const (
	ExecutionReport ReportMessageType = iota
	ErrorReport
)

type Message interface {
	GetType() MessageType
}

// Message format constants
const (
	BaseMessageHeaderLen        = 2
	NewOrderMessageHeaderLen    = 2 + 2 + 4 + 8 + 8 + 1 + 1
	CancelOrderMessageHeaderLen = 2 + 16
)

// Generic message type.
type BaseMessage struct {
	TypeOf MessageType // 2 bytes
}

func (m BaseMessage) GetType() MessageType {
	return m.TypeOf
}

func parseMessage(msg []byte) (Message, error) {
	if len(msg) < BaseMessageHeaderLen {
		return BaseMessage{}, errors.New("message too short to contain header")
	}

	typeOf := MessageType(binary.BigEndian.Uint16(msg[0:2]))
	msg = msg[2:]
	switch typeOf {
	case NewOrder:
		return parseNewOrder(msg)
	case CancelOrder:
		return parseCancelOrder(msg)
	default:
		return BaseMessage{}, ErrInvalidMessageType
	}
}

type NewOrderMessage struct {
	BaseMessage
	AssetType   AssetType // 2 bytes
	OrderType   OrderType // 2 bytes
	Ticker      string    // 4 bytes
	LimitPrice  float64   // 8 bytes
	Quantity    uint64    // 8 bytes
	Side        Side      // 1 byte
	UsernameLen uint8     // 1 byte
	Username    string    // n bytes
}

func (o *NewOrderMessage) Order() (Order, error) {
	orderUUID := uuid.New().String()
	if orderUUID == "" {
		return Order{}, ErrInvalidUUID
	}

	return Order{
		UUID:       orderUUID,
		AssetType:  o.AssetType,
		OrderType:  o.OrderType,
		Ticker:     o.Ticker,
		LimitPrice: o.LimitPrice,
		Quantity:   o.Quantity,
		Side:       o.Side,
		Owner:      o.Username,
	}, nil
}

func parseNewOrder(msg []byte) (NewOrderMessage, error) {
	m := NewOrderMessage{BaseMessage: BaseMessage{TypeOf: NewOrder}}

	m.AssetType = AssetType(binary.BigEndian.Uint16(msg[0:2]))
	m.OrderType = OrderType(binary.BigEndian.Uint16(msg[2:4]))
	m.Ticker = string(msg[4:8]) // Assuming ASCII/UTF-8 string
	m.LimitPrice = math.Float64frombits(binary.BigEndian.Uint64(msg[8:16]))
	m.Quantity = binary.BigEndian.Uint64(msg[16:24])
	m.Side = Side(msg[24])
	m.UsernameLen = uint8(msg[25])

	// Calculate expected total length.
	expectedTotalLen := int(NewOrderMessageHeaderLen + m.UsernameLen)
	if len(msg) < expectedTotalLen {
		return NewOrderMessage{}, ErrMessageTooShort
	}
	m.Username = string(msg[26 : 26+m.UsernameLen])

	return m, nil
}

type CancelOrderMessage struct {
	BaseMessage
	AssetType AssetType // 2 bytes
	OrderUUID string    // 16 bytes
}

func parseCancelOrder(msg []byte) (CancelOrderMessage, error) {
	m := CancelOrderMessage{BaseMessage: BaseMessage{TypeOf: CancelOrder}}

	if len(msg) < CancelOrderMessageHeaderLen {
		return CancelOrderMessage{}, ErrMessageTooShort
	}
	m.AssetType = AssetType(binary.BigEndian.Uint16(msg[0:2]))
	m.OrderUUID = string(msg[2:16])

	return m, nil
}

type Report struct {
	MessageType     ReportMessageType // 1 byte
	AssetType       AssetType         // 1 byte
	Side            Side              // 1 byte
	Timestamp       uint64            // 8 bytes
	Quantity        uint64            // 8 bytes
	Price           float64           // 8 bytes
	CounterpartyLen uint16            // 2 bytes
	ErrStrLen       uint32            // 4 bytes
	Ticker          string            // 4 bytes
	UUID            string            // 16 bytes
	Err             string            // n bytes
	Counterparty    string            // n bytes (in this case we show who)
}

const reportFixedHeaderLen = 1 + 1 + 1 + 8 + 8 + 8 + 2 + 4 + 4 + 16

// Serialize converts the report to be sent on the wire.
func (r *Report) Serialize() ([]byte, error) {
	totalSize := reportFixedHeaderLen + len(r.Err) + len(r.Counterparty)

	buf := make([]byte, totalSize)
	buf[0] = byte(r.MessageType)
	buf[1] = byte(r.AssetType)
	buf[2] = byte(r.Side)
	binary.BigEndian.PutUint64(buf[3:11], r.Timestamp)
	binary.BigEndian.PutUint64(buf[11:19], r.Quantity)
	binary.BigEndian.PutUint64(buf[19:27], math.Float64bits(r.Price))
	binary.BigEndian.PutUint16(buf[27:29], r.CounterpartyLen)
	binary.BigEndian.PutUint32(buf[29:33], r.ErrStrLen)

	// Pack Strings (Ticker and UUID) into fixed buffers
	// copy() ensures we don't panic if strings are shorter.
	copy(buf[33:37], r.Ticker[:4])
	copy(buf[37:53], r.UUID[:16])

	offset := reportFixedHeaderLen
	if r.ErrStrLen > 0 {
		copy(buf[offset:], r.Err)
	}
	offset += int(r.ErrStrLen)
	if r.CounterpartyLen > 0 {
		copy(buf[offset:], r.Counterparty)
	}
	return buf, nil
}

// generateTradeReports generates both trade reports required addressable to
// the respective counterparty.
func generateWireTradeReports(trade Trade, err error) ([]byte, []byte, error) {
	errStr := ""
	if err != nil {
		errStr = fmt.Sprintf("%w", err)
	}

	// Helper to create a report.
	createReport := func(party *Order, counterParty *Order, trade Trade) Report {
		return Report{
			MessageType:     ExecutionReport,
			AssetType:       counterParty.AssetType,
			Side:            party.Side,
			Timestamp:       uint64(trade.Timestamp.Unix()),
			Quantity:        trade.MatchQty,
			Price:           trade.Price,
			CounterpartyLen: uint16(len(counterParty.Owner)),
			ErrStrLen:       uint32(len(errStr)),
			Ticker:          party.Ticker[:4],
			UUID:            party.UUID[:16],
			Counterparty:    counterParty.Owner,
			Err:             errStr,
		}
	}

	// Create struct representations
	r1 := createReport(trade.Party, trade.CounterParty, trade)
	r2 := createReport(trade.CounterParty, trade.Party, trade)

	// Serialize to []byte
	b1, err := r1.Serialize()
	if err != nil {
		return nil, nil, err
	}

	b2, err := r2.Serialize()
	if err != nil {
		return nil, nil, err
	}

	return b1, b2, nil
}

func generateWireErrorReports(err error) ([]byte, error) {
	errStr := fmt.Sprintf("%w", err)
	report := Report{
		MessageType: ErrorReport,
		Timestamp:   uint64(time.Now().UnixNano()),
		ErrStrLen:   uint32(len(errStr)),
		Err:         errStr,
	}
	return report.Serialize()
}
