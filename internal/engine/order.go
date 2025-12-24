package engine

import "time"

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
