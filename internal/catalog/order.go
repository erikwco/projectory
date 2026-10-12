package catalog

// OrderKeys returns the supported orders in their keyboard cycling sequence.
func OrderKeys() []string { return []string{"added", "visited", "visits", "name"} }

func ValidOrder(order string) bool {
	for _, candidate := range OrderKeys() {
		if order == candidate {
			return true
		}
	}
	return false
}

// NextOrder cycles a validated order forward or backward.
func NextOrder(order string, backwards bool) string {
	orders := OrderKeys()
	step := 1
	if backwards {
		step = len(orders) - 1
	}
	for i, candidate := range orders {
		if candidate == order {
			return orders[(i+step)%len(orders)]
		}
	}
	return orders[0]
}
