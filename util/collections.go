package util

// Map the items in an input slice to something else, omitting null values
func Map[J any, K any](in []J, f func(J) *K) []K {
	var k []K

	for _, v := range in {
		o := f(v)
		if o != nil {
			k = append(k, *o)
		}
	}

	return k
}

// Map the items in an input slice to something else, leaving null values
func NullMap[J any, K any](in []J, f func(J) K) []K {
	k := make([]K, len(in))

	for i, v := range in {
		k[i] = f(v)
	}

	return k
}
