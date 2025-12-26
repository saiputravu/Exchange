package tests

import (
	"fenrir/internal/engine"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- Setup & Helpers --------------------------------------------------------

func createTestOrderBook() *engine.OrderBook {
	eng := engine.New(engine.Equities)
	book := eng.Books[engine.Equities]
	return &book
}

// placeTestOrders helps insert a batch of limit orders at a specific price/side.
func placeTestOrders(book *engine.OrderBook, price float64, side engine.Side, quantities ...uint64) error {
	for _, qty := range quantities {
		if err := book.PlaceOrder(engine.Order{
			UUID:          "test-id",
			Side:          side,
			OrderType:     engine.LimitOrder,
			LimitPrice:    price,
			Quantity:      qty,
			TotalQuantity: qty,
		}); err != nil {
			return err
		}
	}
	return nil
}

type Quantity struct {
	quantity      uint64
	totalQuantity uint64
}

// newQuantity creates a quantity with regular and total the same value.
func newQuantity(quantity uint64) Quantity {
	return Quantity{quantity, quantity}
}

// buildExpectedLevel constructs the expected PriceLevel struct to compare against.
func buildExpectedLevel(price float64, side engine.Side, quantities ...Quantity) *engine.PriceLevel {
	orders := make([]*engine.Order, len(quantities))
	for i, qty := range quantities {
		orders[i] = &engine.Order{
			UUID:          "test-id",
			Side:          side,
			OrderType:     engine.LimitOrder,
			LimitPrice:    price,
			Quantity:      qty.quantity,
			TotalQuantity: qty.totalQuantity,
		}
	}
	return &engine.PriceLevel{
		PriceLevel: price,
		Orders:     orders,
	}
}

// sanitizeLevels zeros out timestamps to allow strict struct equality checks.
func sanitizeLevels(levels []*engine.PriceLevel) []*engine.PriceLevel {
	for _, lvl := range levels {
		for _, order := range lvl.Orders {
			order.ExchTimestamp = time.Time{}
		}
	}
	return levels
}

// --- Tests ------------------------------------------------------------------

func TestPlaceOrder_Limit(t *testing.T) {
	book := createTestOrderBook()

	// 1. Setup: Place 3 orders on Buy side and 3 on Sell side
	assert.NoError(t, placeTestOrders(book, 99.0, engine.Buy, 100, 90, 80))
	assert.NoError(t, placeTestOrders(book, 100.0, engine.Sell, 100, 90, 80))

	// 2. Define Expectations
	expectedAsks := []*engine.PriceLevel{
		buildExpectedLevel(
			100.0, engine.Sell, newQuantity(100), newQuantity(90), newQuantity(80),
		),
	}

	expectedBids := []*engine.PriceLevel{
		buildExpectedLevel(
			99.0, engine.Buy, newQuantity(100), newQuantity(90), newQuantity(80),
		),
	}

	// 3. Assertions
	// Note: book.Bids.Items() and book.Asks.Items() are assumed to return []*engine.PriceLevel
	assert.Equal(t, expectedAsks, sanitizeLevels(book.Asks.Items()))
	assert.Equal(t, expectedBids, sanitizeLevels(book.Bids.Items()))
}

func TestPlaceOrder_Limit_MultipleLevels_WithMatch(t *testing.T) {
	book := createTestOrderBook()

	// 1. Setup BIDS: Highest price first (99 -> 98)
	assert.NoError(t, placeTestOrders(book, 99.0, engine.Buy, 100, 90, 80))
	assert.NoError(t, placeTestOrders(book, 98.0, engine.Buy, 50))

	// 2. Setup ASKS: Lowest price first (100 -> 101)
	assert.NoError(t, placeTestOrders(book, 100.0, engine.Sell, 100, 90))
	assert.NoError(t, placeTestOrders(book, 101.0, engine.Sell, 20))

	// 3. Define Expectations
	expectedAsks := []*engine.PriceLevel{
		buildExpectedLevel(100.0, engine.Sell, newQuantity(100), newQuantity(90)),
		buildExpectedLevel(101.0, engine.Sell, newQuantity(20)),
	}

	expectedBids := []*engine.PriceLevel{
		buildExpectedLevel(
			99.0, engine.Buy, newQuantity(100), newQuantity(90), newQuantity(80),
		),
		buildExpectedLevel(98.0, engine.Buy, newQuantity(50)),
	}

	// 4. Assertions
	// Validates that the engine correctly sorts levels based on price priority
	assert.Equal(t, expectedAsks, sanitizeLevels(book.Asks.Items()), "Asks should be sorted Low -> High")
	assert.Equal(t, expectedBids, sanitizeLevels(book.Bids.Items()), "Bids should be sorted High -> Low")

	// 5. Check complete match.
	assert.NoError(t, placeTestOrders(book, 100.0, engine.Buy, 100))
	expectedAsks = []*engine.PriceLevel{
		buildExpectedLevel(100.0, engine.Sell, newQuantity(90)),
		buildExpectedLevel(101.0, engine.Sell, newQuantity(20)),
	}
	assert.Equal(t, expectedAsks, sanitizeLevels(book.Asks.Items()), "Asks should be sorted Low -> High")

	// 6. Check partial match.
	assert.NoError(t, placeTestOrders(book, 100.0, engine.Buy, 20))
	expectedAsks = []*engine.PriceLevel{
		buildExpectedLevel(100.0, engine.Sell, Quantity{70, 90}),
		buildExpectedLevel(101.0, engine.Sell, newQuantity(20)),
	}
	assert.Equal(t, expectedAsks, sanitizeLevels(book.Asks.Items()), "Asks should be sorted Low -> High")
}

func TestPlaceOrder_Limit_MultipleLevels_WithMatchSweep(t *testing.T) {
	book := createTestOrderBook()

	// 1. Setup BIDS: Highest price first (99 -> 98)
	assert.NoError(t, placeTestOrders(book, 99.0, engine.Buy, 100, 90, 80))
	assert.NoError(t, placeTestOrders(book, 98.0, engine.Buy, 50))

	// 2. Setup ASKS: Lowest price first (100 -> 101)
	assert.NoError(t, placeTestOrders(book, 100.0, engine.Sell, 100, 90))
	assert.NoError(t, placeTestOrders(book, 101.0, engine.Sell, 20))

	// 3. Define Expectations
	expectedAsks := []*engine.PriceLevel{
		buildExpectedLevel(100.0, engine.Sell, newQuantity(100), newQuantity(90)),
		buildExpectedLevel(101.0, engine.Sell, newQuantity(20)),
	}

	expectedBids := []*engine.PriceLevel{
		buildExpectedLevel(
			99.0, engine.Buy, newQuantity(100), newQuantity(90), newQuantity(80),
		),
		buildExpectedLevel(98.0, engine.Buy, newQuantity(50)),
	}

	// 4. Assertions
	// Validates that the engine correctly sorts levels based on price priority
	assert.Equal(t, expectedAsks, sanitizeLevels(book.Asks.Items()), "Asks should be sorted Low -> High")
	assert.Equal(t, expectedBids, sanitizeLevels(book.Bids.Items()), "Bids should be sorted High -> Low")

	// 5. Check sweep match.
	assert.NoError(t, placeTestOrders(book, 100.0, engine.Buy, 120))
	expectedAsks = []*engine.PriceLevel{
		buildExpectedLevel(100.0, engine.Sell, Quantity{70, 90}),
		buildExpectedLevel(101.0, engine.Sell, newQuantity(20)),
	}
	assert.Equal(t, expectedAsks, sanitizeLevels(book.Asks.Items()), "Asks should be sorted Low -> High")

	// 6. Check multi-level sweep with a deep into the book order (100.0, 101.0).
	assert.NoError(t, placeTestOrders(book, 103.0, engine.Buy, 80))
	expectedAsks = []*engine.PriceLevel{
		buildExpectedLevel(101.0, engine.Sell, Quantity{10, 20}),
	}
	assert.Equal(t, expectedAsks, sanitizeLevels(book.Asks.Items()), "Asks should be sorted Low -> High")
}
