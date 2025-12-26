package engine

import "time"

// This is the main matchine engine.

type Engine struct {
	Books map[AssetType]OrderBook
}

func New(supportedAssets ...AssetType) *Engine {
	engine := &Engine{
		Books: make(map[AssetType]OrderBook),
	}

	for assetType := range supportedAssets {
		engine.Books[AssetType(assetType)] = NewOrderBook(engine)
	}

	return engine
}

func (engine *Engine) PlaceOrder(assetType AssetType, order Order) {
}

// Match sanity checks before firing an execution report to the
// counterparty and logging an internal trade.
func (engine *Engine) Trade(taker, maker *Order, quantity uint64) {
	// FIXME: Fire an execution report when the reporting is setup
	//        Do this to both parties.
	// FIXME: Log an internal trade, once the historical data ingestion
	//        is setup.
	_ = time.Now()
}
