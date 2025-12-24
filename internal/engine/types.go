package engine

type AssetType int

// TODO: Flesh these out more, if we care.

const (
	Equities AssetType = iota
)

type Side int

const (
	Buy Side = iota
	Sell
)

type OrderType int

const (
	// Limit orders are an order to buy or sell a secuirty at a specified
	// price or better. Limit orders may rest on the order book until
	// filled.
	LimitOrder OrderType = iota
	// Market orders are instructions to buy or sell immediately.
	// This order guarantees that the order will be executed without
	// guarantees on the execution price. A market order will generally
	// execute at or near the current best price .
	MarketOrder
)
