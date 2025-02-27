package order_book

import (
	"fmt"
)

type Product int

const (
	Apple Product = iota
	Nvidia
)

var productName = map[Product]string{
	Apple:  "AAPL",
	Nvidia: "NVDA",
}

func (p Product) String() string {
	return productName[p]
}
