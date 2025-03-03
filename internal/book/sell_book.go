package book

type SellBook []*Order

func (book SellBook) Len() int { return len(book) }

// Ordering function for heap
func (book SellBook) Less(a, b Order) bool {
	if a.price == b.price {
		return a.time.Nanosecond() < b.time.Nanosecond() // Time should be smallest (earliest) first
	}
	return a.price < b.price // Use less than so that the lowest sell order is first
}

func (book SellBook) Swap(i, j int) {
	book[i], book[j] = book[j], book[i]
}

func (book *SellBook) Push(o Order) {
	*book = append(*book, &o)
}

func (book *SellBook) Pop() Order {
	old := *book
	n := len(old)
	o := old[n-1]
	old[n-1] = nil
	*book = old[0 : n-1]
	return *o
}
