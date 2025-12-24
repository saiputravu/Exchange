package engine

import (
	"errors"
	"github.com/tidwall/btree"
	"time"
)

var (
	ErrNotEnoughLiquidity = errors.New("not enough liquidity")
	ErrRejection          = errors.New("order rejection")
)

type PriceLevel struct {
	priceLevel float64
	orders     []*Order
}

type PriceLevels = btree.BTreeG[PriceLevel]
type OrderBook struct {
	// Pointer to the owning engine.
	engine *Engine

	// Price levels to orders sat on the price level, sorted by time added
	// as they will be push-back'd.
	bids *PriceLevels
	asks *PriceLevels

	// Some book keeping
	nBuyOrders   uint64 // Track the number of bids in the book.
	nSellOrders  uint64 // Track the number of asks in the book.
	buyQuantity  uint64 // Track the bid-side liquidity of the book.
	sellQuantity uint64 // Track the ask-side liquidity of the book.
}

func NewOrderBook() OrderBook {
	// Sorted greatest first.
	bids := btree.NewBTreeG(func(a, b PriceLevel) bool {
		return a.priceLevel > b.priceLevel
	})
	// Sorted least first.
	asks := btree.NewBTreeG(func(a, b PriceLevel) bool {
		return a.priceLevel < b.priceLevel
	})
	return OrderBook{
		bids: bids,
		asks: asks,
	}
}

// PlaceOrder places a new order which can either (fully or partially):
// 1. Execute immediately
// 2. Rest in the book
// Returns whether the placement was successful or not.
//
// This method writes the ExchTimestamp of the order
// to note the exact (unix, system) time at which the
// order was placed. We do not care about accuracy of
// the timestamp, just its relativity to other
// timestamps.
func (book *OrderBook) PlaceOrder(order Order) error {
	order.ExchTimestamp = time.Now()

	// These handle internal book-keeping tasks such as book liquidity tracking.
	switch order.OrderType {
	case LimitOrder:
		return book.handleLimit(order)
	case MarketOrder:
		return book.handleMarket(order)
	}
	return nil
}

// handleMarket performs a sweep on the side until volume is filled.
func (book *OrderBook) handleMarket(order Order) error {
	// Sanity check.
	if (order.Side == Buy && book.sellQuantity < order.TotalQuantity) ||
		(order.Side == Sell && book.buyQuantity < order.TotalQuantity) {
		// We do not have enough liquidty to cover the order in the book,
		// we should just give up.
		return ErrNotEnoughLiquidity
	}

	var levels *PriceLevels
	switch order.Side {
	case Buy:
		levels = book.asks
	case Sell:
		levels = book.bids
	}

	// While liquidity left sweep the order book. Keep track of the number of
	// orders we lifted off the book during the sweep for book keeping.
	liftedOrders := uint64(0)
	for order.Quantity > 0 {
		// Min here accounts for bids and asks being in inverse order,
		// based on their comparison method.
		level, ok := levels.Min()
		if !ok {
			// This should not happen, as we have a sanity check.
			// If this happens, something bad has happened.
			return ErrNotEnoughLiquidity
		}

		var i int
		var counterOrder *Order
		for i, counterOrder = range level.orders {
			// Consume order as much as possible.
			diff := min(order.Quantity, counterOrder.Quantity)
			order.Quantity -= diff
			counterOrder.Quantity -= diff
			book.engine.Match(&order, counterOrder, diff)
			if counterOrder.Quantity == 0 {
				liftedOrders++
			}
		}

		// Either we got to the end or not. If we have gotten to the
		// end then we can delete this entry. Else, we should resize
		// the orders.
		if i == len(level.orders)-1 && counterOrder.Quantity == 0 {
			levels.Delete(level)
		} else {
			// Remove all preceeding orders.
			level.orders = level.orders[i:]
		}
	}

	// Bookkeeping
	switch order.Side {
	case Buy:
		book.sellQuantity -= order.TotalQuantity
		book.nSellOrders -= liftedOrders
	case Sell:
		book.buyQuantity -= order.TotalQuantity
		book.nBuyOrders -= liftedOrders
	}

	return nil
}

func (book *OrderBook) handleLimit(order Order) error {
	return nil
}
