package engine

import (
	"errors"
	"time"

	"github.com/tidwall/btree"

	. "fenrir/internal/common"
)

var (
	ErrNotEnoughLiquidity = errors.New("not enough liquidity")
	ErrRejection          = errors.New("order rejection")
)

// OrderAsc sorts orders by time priority (FIFO).
// If timestamps are equal, it falls back to UUID for stability.
func OrderAsc(a, b *Order) bool {
	if a.ExchTimestamp.Before(b.ExchTimestamp) {
		return true
	}
	if a.ExchTimestamp.After(b.ExchTimestamp) {
		return false
	}
	return a.UUID < b.UUID
}

type PriceLevel struct {
	PriceLevel float64
	Orders     *btree.BTreeG[*Order]
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

func (book *OrderBook) CancelOrder(uuid string) error {
	// FIXME: implement this
	return nil
}

type FlatPriceLevel struct {
	PriceLevel float64
	Orders     []*Order
}

// flattenLevels converts the complex BTree structure into a simple slice
// so we can use assert.Equal easily.
func FlattenLevels(levels []*PriceLevel) []FlatPriceLevel {
	var out []FlatPriceLevel
	for _, lvl := range levels {
		var orders []*Order
		lvl.Orders.Scan(func(item *Order) bool {
			// Copy out the order so to not affect true value which will mess with
			// ordering.
			// Zero out timestamps for strict equality checking in tests
			order := *item
			order.ExchTimestamp = time.Time{}
			orders = append(orders, &order)
			return true
		})
		out = append(out, FlatPriceLevel{
			PriceLevel: lvl.PriceLevel,
			Orders:     orders,
		})
	}
	return out
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
	var errs []error
	for {
		bestBid, bidOk := book.Bids.MinMut()
		bestAsk, askOk := book.Asks.MinMut()

		// If either side is empty, or prices don't cross, we are done.
		if !bidOk || !askOk || bestBid.PriceLevel < bestAsk.PriceLevel {
			break
		}

		// While there are still orders on either side, move forward on the orders.
		var aIdx, bIdx int
		for aIdx < bestAsk.Orders.Len() && bIdx < bestBid.Orders.Len() {
			askOrder, _ := bestAsk.Orders.MinMut()
			bidOrder, _ := bestBid.Orders.MinMut()

			matchQty := min(askOrder.Quantity, bidOrder.Quantity)
			askOrder.Quantity -= matchQty
			bidOrder.Quantity -= matchQty

			// Call the trade engine. Taker and maker is decided by whose order was
			// received first. The earlier order must be resting. It is expected
			// that, if there is functionality ot change order details at a later
			// date, then we still consider the new order taker.
			//
			// The price is matched at maker's price level.
			if askOrder.ExchTimestamp.After(bidOrder.ExchTimestamp) {
				if err := book.engine.DoTrade(askOrder, bidOrder, bestBid.PriceLevel, matchQty); err != nil {
					errs = append(errs, err)
				}
			} else {
				if err := book.engine.DoTrade(bidOrder, askOrder, bestAsk.PriceLevel, matchQty); err != nil {
					errs = append(errs, err)
				}
			}

			// Remove order from book if it is completelly filled.
			if askOrder.Quantity == 0 {
				bestAsk.Orders.Delete(askOrder)
			}
			if bidOrder.Quantity == 0 {
				bestBid.Orders.Delete(bidOrder)
			}
		}

		// Full consumption cases (i.e. empty levels).
		if bestAsk.Orders.Len() == 0 {
			book.Asks.Delete(bestAsk)
		}
		if bestBid.Orders.Len() == 0 {
			book.Bids.Delete(bestBid)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
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

		level.Orders.DeleteAscend(nil, func(restingOrder *Order) btree.Action {
			// Give up if the original order is filled fully.
			if order.Quantity <= 0 {
				return btree.Stop
			}

			matchQty := min(order.Quantity, restingOrder.Quantity)
			order.Quantity -= matchQty
			restingOrder.Quantity -= matchQty

			// Consume order as much as possible and book trade, passing
			// the taker and maker.
			book.engine.DoTrade(&order, restingOrder, level.PriceLevel, matchQty)

			if restingOrder.Quantity == 0 {
				liftedOrders++
				return btree.Delete
			}
			return btree.Keep
		})

		// If orders are empty, delete the price level.
		if level.Orders.Len() == 0 {
			levels.Delete(level)
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
	if !ok {
		level = &PriceLevel{
			PriceLevel: order.LimitPrice,
			Orders:     btree.NewBTreeG(OrderAsc),
		}
		levels.Set(level)
	}
	level.Orders.Set(&order)

	// Trigger the matching.
	return book.Match()
}
