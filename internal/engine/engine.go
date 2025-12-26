package engine

import (
	"errors"
	. "fenrir/internal/common"
	"time"
)

var (
	ErrBookNotFound = errors.New("order book not found")
)

// A reporter deals with passing a trade up to the respective owners.
type Reporter interface {
	Report(owner string, trade Trade) error
}

// This is the main matchine engine.
type Engine struct {
	Books    map[AssetType]OrderBook
	Trades   []Trade
	reporter Reporter
}

func New(reporter Reporter, supportedAssets ...AssetType) *Engine {
	engine := &Engine{
		Books:    make(map[AssetType]OrderBook),
		reporter: reporter,
	}

	for assetType := range supportedAssets {
		engine.Books[AssetType(assetType)] = NewOrderBook(engine)
	}

	return engine
}

func (engine *Engine) PlaceOrder(assetType AssetType, order Order) error {
	book, ok := engine.Books[assetType]
	if !ok {
		return ErrBookNotFound
	}
	return book.PlaceOrder(order)
}

func (engine *Engine) CancelOrder(assetType AssetType, uuid string) error {
	book, ok := engine.Books[assetType]
	if !ok {
		return ErrBookNotFound
	}
	return book.CancelOrder(uuid)
}

// Match sanity checks before firing an execution report to the
// counterparty and logging an internal trade.
// We expect the price the trade was matched (maker's price level)
// and quantity matched.
func (engine *Engine) DoTrade(taker, maker *Order, price float64, quantity uint64) error {
	trade := Trade{
		Party:        taker,
		CounterParty: maker,
		Timestamp:    time.Now(),
		MatchQty:     quantity,
		Price:        price,
	}

	if err := engine.reporter.Report(taker.Owner, trade); err != nil {
		return err
	}
	if err := engine.reporter.Report(maker.Owner, trade); err != nil {
		return err
	}

	// TODO: Think about persistance but I cba right now.
	engine.Trades = append(engine.Trades, trade)
	return nil
}
