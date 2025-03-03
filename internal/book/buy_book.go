package book

type BuyBook []*Order

func (book BuyBook) Len() int { return len(book) }

// Ordering function for heap
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
