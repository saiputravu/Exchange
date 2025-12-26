package engine

import (
	"errors"
	. "fenrir/internal/common"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	ErrBookNotFound = errors.New("order book not found")
)

// A reporter deals with passing a trade up to the respective owners.
type Reporter interface {
	ReportTrade(trade Trade, err error) error
	ReportError(client string, err error) error
}

// This is the main matchine engine.
type Engine struct {
	Books    map[AssetType]OrderBook
	Trades   []Trade
	reporter Reporter
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

func (engine *Engine) SetReporter(reporter Reporter) {
	engine.reporter = reporter
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

	if err := engine.reporter.ReportTrade(trade, nil); err != nil {
		return err
	}
	if err := engine.reporter.ReportTrade(trade, nil); err != nil {
		return err
	}

	// TODO: Think about persistance but I cba right now.
	engine.Trades = append(engine.Trades, trade)
	return nil
}

func (engine *Engine) LogBook() {
	for asset, book := range engine.Books {
		bids := FlattenLevels(book.Bids.Items())
		asks := FlattenLevels(book.Asks.Items())
		log.Info().
			Int("asset", int(asset)).
			Any("bids", bids).
			Any("asks", asks).
			Msg("")
	}
}
