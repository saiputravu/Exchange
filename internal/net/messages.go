package net

type MessageType int

const (
	Heartbeat MessageType = iota
	Limit
)

type Message struct {
	typeOf MessageType
	// FIXME: Implement this.
	field string
}

func parseMessage(msg []byte) (Message, error) {
	// FIXME: Implement this.
	return Message{typeOf: Heartbeat, field: string(msg)}, nil
}
