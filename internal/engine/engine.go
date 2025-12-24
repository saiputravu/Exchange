package engine

import "time"

// This is the main matchine engine.

type Engine struct {
	books map[AssetType]OrderBook
}

func New(supportedAssets ...AssetType) *Engine {
	engine := &Engine{
		books: make(map[AssetType]OrderBook),
	}

	for assetType := range supportedAssets {
		engine.books[AssetType(assetType)] = NewOrderBook()
	}

	return engine
}

func (engine *Engine) AddOrder(order Order) {
}

// Match sanity checks before firing an execution report to the
// counterparty and logging an internal trade.
func (engine *Engine) Match(order, counter *Order, quantity uint64) {
	// FIXME: Fire an execution report when the reporting is setup
	//        Do this to both parties.
	// FIXME: Log an internal trade, once the historical data ingestion
	//        is setup.
	_ = time.Now()
}
