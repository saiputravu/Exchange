package book

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
type Book map[string]ProductBook

func (b Book) InsertOrder() {

}
