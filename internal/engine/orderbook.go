package engine

import (
	"errors"
	"time"

	"github.com/tidwall/btree"
)

var (
	ErrNotEnoughLiquidity = errors.New("not enough liquidity")
	ErrRejection          = errors.New("order rejection")
)

type PriceLevel struct {
	PriceLevel float64
	Orders     []*Order
}

type PriceLevels = btree.BTreeG[*PriceLevel]
type OrderBook struct {
	// Pointer to the owning engine.
	engine *Engine

	// Price levels to orders sat on the price level, sorted by time added
	// as they will be push-back'd.
	Bids *PriceLevels
	Asks *PriceLevels

	// Some book keeping
	nBuyOrders   uint64 // Track the number of bids in the book.
	nSellOrders  uint64 // Track the number of asks in the book.
	buyQuantity  uint64 // Track the bid-side liquidity of the book.
	sellQuantity uint64 // Track the ask-side liquidity of the book.
}

func NewOrderBook(engine *Engine) OrderBook {
	// Sorted greatest first.
	bids := btree.NewBTreeG(func(a, b *PriceLevel) bool {
		return a.PriceLevel > b.PriceLevel
	})
	// Sorted least first.
	asks := btree.NewBTreeG(func(a, b *PriceLevel) bool {
		return a.PriceLevel < b.PriceLevel
	})
	return OrderBook{
		engine: engine,
		Bids:   bids,
		Asks:   asks,
	}
}

// PlaceOrder places a new order which can either (fully or partially):
// 1. Execute immediately
// 2. Rest in the book
// Returns whether the placement was successful or not.
//
// This method writes the ExchTimestamp of the order to note the exact (unix, system)
// time at which the order was placed. We do not care about the accuracy of the
// timestamp, just its relativity to other timestamps.
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

// Match consumes the top of book price levels while they cross (i.e., bid >= ask).
// While these orders cross, we match orders in price-time-priority.
//
// The order that triggered the matching, if there is a cross, is considered to be
// a liquidity taker. Otherwise, resting orders are considered liquidity makers. If
// an order is both (i.e., a partial fill), then we consider taker fees only on the
// partial quantity.
//
// NOTE: There will only be a matching, if the new order's limit price is top of book.
// Otherwise, we would have a stable state.
func (book *OrderBook) Match() error {
	// Consume crossing orders. This will essentially be our latest order sweeping
	// across priceLevels as far as its depth and liquidity go.
	for {
		bestBid, bidOk := book.Bids.MinMut()
		bestAsk, askOk := book.Asks.MinMut()

		// If either side is empty, or prices don't cross, we are done.
		if !bidOk || !askOk || bestBid.PriceLevel < bestAsk.PriceLevel {
			break
		}

		// While there are still orders on either side, move forward on the orders.
		var aIdx, bIdx int
		for aIdx < len(bestAsk.Orders) && bIdx < len(bestBid.Orders) {
			askOrder := bestAsk.Orders[aIdx]
			bidOrder := bestBid.Orders[bIdx]

			matchQty := min(askOrder.Quantity, bidOrder.Quantity)
			askOrder.Quantity -= matchQty
			bidOrder.Quantity -= matchQty

			// Call the trade engine. Taker and maker is decided by whose order was
			// received first. The earlier order must be resting. It is expected
			// that, if there is functionality ot change order details at a later
			// date, then we still consider the new order taker.
			if askOrder.ExchTimestamp.After(bidOrder.ExchTimestamp) {
				book.engine.Trade(askOrder, bidOrder, matchQty)
			} else {
				book.engine.Trade(bidOrder, askOrder, matchQty)
			}

			// Move forward
			if askOrder.Quantity == 0 {
				aIdx++
			}
			if bidOrder.Quantity == 0 {
				bIdx++
			}
		}

		// If we are here, done one or more of the following:
		// 1. We have partially or fully consumed a price level.
		// 2. We have depleted the remaining order quantity (i.e. no more matches).
		//
		// Case 2 is handled on the re-loop. We handle case 1.
		if aIdx > 0 {
			bestAsk.Orders = bestAsk.Orders[aIdx:]
		}
		if bIdx > 0 {
			bestBid.Orders = bestBid.Orders[bIdx:]
		}
		// Full consumption cases (i.e. empty levels).
		if len(bestAsk.Orders) == 0 {
			book.Asks.Delete(bestAsk)
		}
		if len(bestBid.Orders) == 0 {
			book.Bids.Delete(bestBid)
		}
	}
	return nil
}

// handleMarket handles a market order. Performs a sweep on the side until volume is
// filled. Market orders are always liquidity takers.
func (book *OrderBook) handleMarket(order Order) error {
	// FIXME: figure out how to assign fees.
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
		levels = book.Asks
	case Sell:
		levels = book.Bids
	}

	// While liquidity left sweep the order book. Keep track of the number of orders
	// we lifted off the book during the sweep for book keeping.
	liftedOrders := uint64(0)
	for order.Quantity > 0 {
		// Min here accounts for bids and asks being in inverse order, based on their
		// comparison method.
		level, ok := levels.MinMut()
		if !ok {
			// This should not happen, as we have a sanity check.
			// If this happens, something bad has happened.
			return ErrNotEnoughLiquidity
		}

		var i int
		var restingOrder *Order
		for i, restingOrder = range level.Orders {
			matchQty := min(order.Quantity, restingOrder.Quantity)
			order.Quantity -= matchQty
			restingOrder.Quantity -= matchQty

			// Consume order as much as possible and book trade, passing
			// the taker and maker.
			book.engine.Trade(&order, restingOrder, matchQty)

			if restingOrder.Quantity == 0 {
				liftedOrders++
			}

			// Break out if we have filled the liquidity quota
			if order.Quantity == 0 {
				break
			}
		}

		// Resizing Logic
		if restingOrder.Quantity == 0 {
			// If the last order we touched is empty, we consumed it.
			// If we consumed the whole level (i is the last index), delete level.
			if i == len(level.Orders)-1 {
				levels.Delete(level)
			} else {
				// Otherwise, slice off the consumed orders (0 to i)
				level.Orders = level.Orders[i+1:]
			}
		} else {
			// We partially filled 'restingOrder' .
			// We remove all orders strictly *before* i.
			level.Orders = level.Orders[i:]
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

// handleLimit handles a limit order. The order is placed at the price level specified
// (tick size handling is assumed to have already been done). This method triggers a
// "matching", which checks for any crossing pairs of orders, which are matched away.
func (book *OrderBook) handleLimit(order Order) error {
	// Limit orders are placed on the same side as their order.Side. This is because
	// they are resting.
	var levels *PriceLevels
	switch order.Side {
	case Buy:
		levels = book.Bids
	case Sell:
		levels = book.Asks
	}

	// TODO: Should probably do some validation on rejecting orders that are too far
	//       away from the top-of-book or too far away from bottom-of-book. To do this
	//       we need to keep track of a per-asset-type tick size. This is too much
	//       effort for me right now.

	// Levels comparator only accounts for price levels, so we create a dummy price
	// level for the search.
	level, ok := levels.GetMut(&PriceLevel{PriceLevel: order.LimitPrice})
	if ok {
		// If the price level already exists, just append onto the existing orders.
		level.Orders = append(level.Orders, &order)
	} else {
		// Otherwise, if the price level does not exist, create the price level.
		levels.Set(&PriceLevel{
			PriceLevel: order.LimitPrice,
			Orders:     []*Order{&order},
		})
	}

	// Trigger the matching.
	return book.Match()
}
