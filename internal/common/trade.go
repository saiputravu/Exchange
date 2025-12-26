package common

import (
	"fmt"
	"time"
)

// Trade accounts for the two parties who matched.
type Trade struct {
	Party        *Order
	CounterParty *Order
	Timestamp    time.Time
	MatchQty     uint64
	Price        float64
}

func (t Trade) String() string {
	return fmt.Sprintf(
		`Party: [
%s]
CounterParty:   [
%s]
Timestamp:      %v
MatchQty:       %d
Price:          %f`,
		t.Party.String(),
		t.CounterParty.String(),
		t.Timestamp.Format(time.RFC3339),
		t.MatchQty,
		t.Price,
	)
}
