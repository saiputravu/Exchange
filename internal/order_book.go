package order_book

import (
	"fmt"
)

type Product int

// NOTE: might want to compare with `Float` from `math/big`: more precise but slower
// NOTE: unsure if want to store price in struct or DDD array style
type Product struct {
	uid    uint16
	ticker string
	price  float64
}

func (p Product) String() string {
	return p.ticker
}

var orderbook = map[Product][]Order{}
