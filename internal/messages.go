package server

type MessageType int

const (
	Limit MessageType = iota
)

type Message struct {
	typeOf MessageType
	// FIXME: Implement this.
	field string
}

func parseMessage(msg []byte) (Message, error) {
	// FIXME: Implement this.
	return Message{typeOf: Limit, field: string(msg)}, nil
}
