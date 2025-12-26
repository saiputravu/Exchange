package net

import (
	"encoding/binary"
	"errors"
	. "fenrir/internal/common"
	"math"
)

var (
	ErrInvalidMessageType = errors.New("invalid message type")
	ErrMessageTooShort    = errors.New("message too short for specified username length")
)

type MessageType int

const (
	Heartbeat MessageType = iota
	NewOrder
	CancelOrder
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
	Quantity    float64   // 8 bytes
	Side        Side      // 1 byte
	UsernameLen uint8     // 1 byte
	Username    string    // n bytes
}

func parseNewOrder(msg []byte) (NewOrderMessage, error) {
	m := NewOrderMessage{BaseMessage: BaseMessage{TypeOf: NewOrder}}

	m.AssetType = AssetType(binary.BigEndian.Uint16(msg[0:2]))
	m.OrderType = OrderType(binary.BigEndian.Uint16(msg[2:4]))
	m.Ticker = string(msg[4:8]) // Assuming ASCII/UTF-8 string
	m.LimitPrice = math.Float64frombits(binary.BigEndian.Uint64(msg[8:16]))
	m.Quantity = math.Float64frombits(binary.BigEndian.Uint64(msg[16:24]))
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
