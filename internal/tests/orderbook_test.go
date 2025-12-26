package tests

import (
	. "fenrir/internal/common"
	"fenrir/internal/engine"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// --- Setup & Helpers --------------------------------------------------------

type MockReporter struct{}

func (r *MockReporter) ReportTrade(trade Trade, err error) error {
	return nil
}

func (r *MockReporter) ReportError(client string, err error) error {
	return nil
}

func createTestOrderBook() *engine.OrderBook {
	eng := engine.New(Equities)
	eng.SetReporter(&MockReporter{})
	book := eng.Books[Equities]
	return &book
}

func placeTestOrders(book *engine.OrderBook, price float64, side Side, quantities ...uint64) error {
	for _, qty := range quantities {
		// Sleep strictly ensures timestamps differ for deterministic FIFO tests
		time.Sleep(1 * time.Nanosecond)
		if err := book.PlaceOrder(Order{
			UUID:          "test-id",
			Side:          side,
			OrderType:     LimitOrder,
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
func buildExpectedLevel(price float64, side Side, quantities ...Quantity) engine.FlatPriceLevel {
	orders := make([]*Order, len(quantities))
	for i, qty := range quantities {
		orders[i] = &Order{
			UUID:          "test-id",
			Side:          side,
			OrderType:     LimitOrder,
			LimitPrice:    price,
			Quantity:      qty.quantity,
			TotalQuantity: qty.totalQuantity,
		}
	}
	return engine.FlatPriceLevel{
		PriceLevel: price,
		Orders:     orders,
	}
}

// --- Tests ------------------------------------------------------------------

func TestPlaceOrder_Limit(t *testing.T) {
	book := createTestOrderBook()

	// 1. Setup: Place 3 orders on Buy side and 3 on Sell side
	assert.NoError(t, placeTestOrders(book, 99.0, Buy, 100, 90, 80))
	assert.NoError(t, placeTestOrders(book, 100.0, Sell, 100, 90, 80))

	// 2. Define Expectations
	expectedAsks := []engine.FlatPriceLevel{
		buildExpectedLevel(
			100.0, Sell, newQuantity(100), newQuantity(90), newQuantity(80),
		),
	}

	expectedBids := []engine.FlatPriceLevel{
		buildExpectedLevel(
			99.0, Buy, newQuantity(100), newQuantity(90), newQuantity(80),
		),
	}

	// 3. Assertions
	// Note: book.Bids.Items() and book.Asks.Items() are assumed to return []*engine.PriceLevel
	assert.Equal(t, expectedAsks, engine.FlattenLevels(book.Asks.Items()))
	assert.Equal(t, expectedBids, engine.FlattenLevels(book.Bids.Items()))
}

func TestPlaceOrder_Limit_MultipleLevels_WithMatch(t *testing.T) {
	book := createTestOrderBook()

	// 1. Setup BIDS: Highest price first (99 -> 98)
	assert.NoError(t, placeTestOrders(book, 99.0, Buy, 100, 90, 80))
	assert.NoError(t, placeTestOrders(book, 98.0, Buy, 50))

	// 2. Setup ASKS: Lowest price first (100 -> 101)
	assert.NoError(t, placeTestOrders(book, 100.0, Sell, 100, 90))
	assert.NoError(t, placeTestOrders(book, 101.0, Sell, 20))

	// 3. Define Expectations
	expectedAsks := []engine.FlatPriceLevel{
		buildExpectedLevel(100.0, Sell, newQuantity(100), newQuantity(90)),
		buildExpectedLevel(101.0, Sell, newQuantity(20)),
	}

	expectedBids := []engine.FlatPriceLevel{
		buildExpectedLevel(
			99.0, Buy, newQuantity(100), newQuantity(90), newQuantity(80),
		),
		buildExpectedLevel(98.0, Buy, newQuantity(50)),
	}

	// 4. Assertions
	// Validates that the engine correctly sorts levels based on price priority
	assert.Equal(t, expectedAsks, engine.FlattenLevels(book.Asks.Items()), "Asks should be sorted Low -> High")
	assert.Equal(t, expectedBids, engine.FlattenLevels(book.Bids.Items()), "Bids should be sorted High -> Low")

	// 5. Check complete match.
	assert.NoError(t, placeTestOrders(book, 100.0, Buy, 100))
	expectedAsks = []engine.FlatPriceLevel{
		buildExpectedLevel(100.0, Sell, newQuantity(90)),
		buildExpectedLevel(101.0, Sell, newQuantity(20)),
	}
	assert.Equal(t, expectedAsks, engine.FlattenLevels(book.Asks.Items()), "Asks should be sorted Low -> High")

	// 6. Check partial match.
	assert.NoError(t, placeTestOrders(book, 100.0, Buy, 20))
	expectedAsks = []engine.FlatPriceLevel{
		buildExpectedLevel(100.0, Sell, Quantity{70, 90}),
		buildExpectedLevel(101.0, Sell, newQuantity(20)),
	}
	assert.Equal(t, expectedAsks, engine.FlattenLevels(book.Asks.Items()), "Asks should be sorted Low -> High")
}

func TestPlaceOrder_Limit_MultipleLevels_WithMatchSweep_Bid(t *testing.T) {
	book := createTestOrderBook()

	// 1. Setup BIDS: Highest price first (99 -> 98)
	assert.NoError(t, placeTestOrders(book, 99.0, Buy, 100, 90, 80))
	assert.NoError(t, placeTestOrders(book, 98.0, Buy, 50))

	// 2. Setup ASKS: Lowest price first (100 -> 101)
	assert.NoError(t, placeTestOrders(book, 100.0, Sell, 100, 90))
	assert.NoError(t, placeTestOrders(book, 101.0, Sell, 20))

	// 3. Define Expectations
	expectedAsks := []engine.FlatPriceLevel{
		buildExpectedLevel(100.0, Sell, newQuantity(100), newQuantity(90)),
		buildExpectedLevel(101.0, Sell, newQuantity(20)),
	}

	expectedBids := []engine.FlatPriceLevel{
		buildExpectedLevel(
			99.0, Buy, newQuantity(100), newQuantity(90), newQuantity(80),
		),
		buildExpectedLevel(98.0, Buy, newQuantity(50)),
	}

	// 4. Assertions
	// Validates that the engine correctly sorts levels based on price priority
	assert.Equal(t, expectedAsks, engine.FlattenLevels(book.Asks.Items()), "Asks should be sorted Low -> High")
	assert.Equal(t, expectedBids, engine.FlattenLevels(book.Bids.Items()), "Bids should be sorted High -> Low")

	// 5. Check sweep match.
	assert.NoError(t, placeTestOrders(book, 100.0, Buy, 120))
	expectedAsks = []engine.FlatPriceLevel{
		buildExpectedLevel(100.0, Sell, Quantity{70, 90}),
		buildExpectedLevel(101.0, Sell, newQuantity(20)),
	}
	assert.Equal(t, expectedAsks, engine.FlattenLevels(book.Asks.Items()), "Asks should be sorted Low -> High")

	// 6. Check multi-level sweep with a deep into the book order (100.0, 101.0).
	assert.NoError(t, placeTestOrders(book, 103.0, Buy, 80))
	expectedAsks = []engine.FlatPriceLevel{
		buildExpectedLevel(101.0, Sell, Quantity{10, 20}),
	}
	assert.Equal(t, expectedAsks, engine.FlattenLevels(book.Asks.Items()), "Asks should be sorted Low -> High")
}

func TestPlaceOrder_Limit_MultipleLevels_WithMatchSweep_Ask(t *testing.T) {
	book := createTestOrderBook()

	// 1. Setup BIDS: Highest price first (99 -> 98)
	assert.NoError(t, placeTestOrders(book, 99.0, Buy, 100, 90, 80))
	assert.NoError(t, placeTestOrders(book, 98.0, Buy, 50))

	// 2. Setup ASKS: Lowest price first (100 -> 101)
	assert.NoError(t, placeTestOrders(book, 100.0, Sell, 100, 90))
	assert.NoError(t, placeTestOrders(book, 101.0, Sell, 20))

	// 3. Define Expectations
	expectedAsks := []engine.FlatPriceLevel{
		buildExpectedLevel(100.0, Sell, newQuantity(100), newQuantity(90)),
		buildExpectedLevel(101.0, Sell, newQuantity(20)),
	}

	expectedBids := []engine.FlatPriceLevel{
		buildExpectedLevel(
			99.0, Buy, newQuantity(100), newQuantity(90), newQuantity(80),
		),
		buildExpectedLevel(98.0, Buy, newQuantity(50)),
	}

	// 4. Assertions
	// Validates that the engine correctly sorts levels based on price priority
	assert.Equal(t, expectedAsks, engine.FlattenLevels(book.Asks.Items()), "Asks should be sorted Low -> High")
	assert.Equal(t, expectedBids, engine.FlattenLevels(book.Bids.Items()), "Bids should be sorted High -> Low")

	// 5. Check sweep match.
	assert.NoError(t, placeTestOrders(book, 96.0, Sell, 310))
	expectedBids = []engine.FlatPriceLevel{
		buildExpectedLevel(98.0, Buy, Quantity{10, 50}),
	}
	assert.Equal(t, expectedBids, engine.FlattenLevels(book.Bids.Items()), "Asks should be sorted Low -> High")
}
