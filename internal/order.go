package order

import (
	"time"
)

type Side int

// See: https://go.dev/ref/spec#Iota
const (
	buy Side = iota
	sell
)

// NOTE might want to compare with `Float` from `math/big`: more precise but slower
type Order struct {
	side    Side
	price   float64
	volume  uint32
	product string
	time    time.Time
	id      uint64
}
