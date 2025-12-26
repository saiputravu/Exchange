package common

import (
	"fmt"
	"time"
)

type Order struct {
	UUID          string    // Order tracked uuid
	AssetType     AssetType //
	OrderType     OrderType //
	Ticker        string    // Specific asset identifier
	Side          Side      // Order side
	LimitPrice    float64   // Limiting price
	Quantity      uint64    // Remaining quantity
	TotalQuantity uint64    // Total volume requested
	Timestamp     time.Time // Time of arrival of order
	ExchTimestamp time.Time // Time of arrival of order into the book
	Owner         string    // Who ownes this order
}

func (order Order) String() string {
	return fmt.Sprintf(
		`UUID:          %v
AssetType:     %v
OrderType:     %v
Ticker:        %s
Side:          %v
LimitPrice:    %f
Quantity:      %d (Total: %d)
Timestamp:     %v
ExchTimestamp: %v
Owner:         %s`,
		order.UUID,
		order.AssetType,
		order.OrderType,
		order.Ticker,
		order.Side,
		order.LimitPrice,
		order.Quantity,
		order.TotalQuantity,
		order.Timestamp.Format(time.RFC3339), // Formatted for readability
		order.ExchTimestamp.Format(time.RFC3339),
		order.Owner,
	)
}
