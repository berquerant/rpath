package rpath

// FindClosestFloor finds the largest element of xs not exceeding target.
//
// xs must be sorted in increasing order, defined by cmp.
// if cmp(a, b) < 0 then value of a < b and cmp(a, b) >= 0 then value of a >= b.
func FindClosestFloor[S ~[]E, E, T any](
	xs S,
	target T,
	cmp func(E, T) int,
) (int, bool) {
	var (
		index int
		found bool
	)

	for i, x := range xs {
		if cmp(x, target) < 0 {
			found = true
			index = i
			continue
		}
		break
	}

	return index, found
}

// FindClosestCeiling finds the smallest element of xs not less than target.
//
// xs must be sorted in increasing order, defined by cmp.
// if cmp(a, b) < 0 then value of a < b and cmp(a, b) >= 0 then value of a >= b.
func FindClosestCeiling[S ~[]E, E, T any](
	xs S,
	target T,
	cmp func(E, T) int,
) (int, bool) {
	var (
		index int
		found bool
	)
	for i := len(xs) - 1; i >= 0; i-- {
		x := xs[i]
		if cmp(x, target) > 0 {
			found = true
			index = i
			continue
		}
		break
	}

	return index, found
}
