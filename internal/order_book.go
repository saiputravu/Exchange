package order

import (
	"container/heap"
)

// NOTE: might want to compare with `Float` from `math/big`: more precise but slower
// NOTE: unsure if want to store price in struct or DDD style array separately
type Product struct {
	uid    uint16
	ticker string
	price  float64
}

func (p Product) String() string {
	return p.ticker
}

// Each product has a book: buy and sell orders
type ProductBook struct {
	buys  BuyBook
	sells SellBook
}

// Our whole book is a collection of ProductBooks, indexed by Product ID
type Book map[uint16]ProductBook

type BuyBook []*Order
type SellBook []*Order

func (book BuyBook) Len() int { return len(book) }

func (book BuyBook) Less(a, b Order) bool {
	if a.price == b.price {
		return a.time.Nanosecond() < b.time.Nanosecond() // Time should be smallest (earliest) first
	}
	return a.price > b.price // Use greater than so that the highest buy order is first
}

func (book BuyBook) Swap(i, j int) {
	book[i], book[j] = book[j], book[i]
}

func (book *BuyBook) Push(o Order) {
	*book = append(*book, &o)
}

func (book *BuyBook) Pop() Order {
	old := *book
	n := len(old)
	o := old[n-1]
	old[n-1] = nil
	*book = old[0 : n-1]
	return *o
}
