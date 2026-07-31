package surrealql

import "github.com/surrealdb/surrealdb.go/pkg/models"

// RangeOpen returns a fully open range (`..`): both sides have no limit.
// Useful as a high sentinel inside array-style record ids, for example
// []any{"London", RangeOpen()}.
func RangeOpen() models.Range[any, models.BoundIncluded[any], models.BoundIncluded[any]] {
	return models.Range[any, models.BoundIncluded[any], models.BoundIncluded[any]]{}
}

// RangeOpenBeginInclusive returns a range that starts at t (inclusive) with
// no upper limit (`t..`).
func RangeOpenBeginInclusive[T any](t T) models.Range[T, models.BoundIncluded[T], models.BoundIncluded[T]] {
	return models.Range[T, models.BoundIncluded[T], models.BoundIncluded[T]]{
		Begin: &models.BoundIncluded[T]{Value: t},
	}
}

// RangeOpenBeginExclusive returns a range that starts after t (exclusive) with
// no upper limit (`t>..`).
func RangeOpenBeginExclusive[T any](t T) models.Range[T, models.BoundExcluded[T], models.BoundIncluded[T]] {
	return models.Range[T, models.BoundExcluded[T], models.BoundIncluded[T]]{
		Begin: &models.BoundExcluded[T]{Value: t},
	}
}

// RangeOpenEndInclusive returns a range with no lower limit up to and including
// t (`..=t`).
func RangeOpenEndInclusive[T any](t T) models.Range[T, models.BoundIncluded[T], models.BoundIncluded[T]] {
	return models.Range[T, models.BoundIncluded[T], models.BoundIncluded[T]]{
		End: &models.BoundIncluded[T]{Value: t},
	}
}

// RangeOpenEndExclusive returns a range with no lower limit up to but not
// including t (`..t`).
func RangeOpenEndExclusive[T any](t T) models.Range[T, models.BoundIncluded[T], models.BoundExcluded[T]] {
	return models.Range[T, models.BoundIncluded[T], models.BoundExcluded[T]]{
		End: &models.BoundExcluded[T]{Value: t},
	}
}

// RangeClosed returns a closed range with both ends inclusive (`a..=b`).
// a and b may be different Go types. Pass nil for a bound value of NULL
// (that is not the same as leaving a side open — use the RangeOpen* helpers
// for open sides).
func RangeClosed(a, b any) models.Range[any, models.BoundIncluded[any], models.BoundIncluded[any]] {
	return models.Range[any, models.BoundIncluded[any], models.BoundIncluded[any]]{
		Begin: &models.BoundIncluded[any]{Value: a},
		End:   &models.BoundIncluded[any]{Value: b},
	}
}

// RangeClosedEndExclusive returns a range from a (inclusive) to b (exclusive)
// (`a..b`). a and b may be different Go types.
func RangeClosedEndExclusive(a, b any) models.Range[any, models.BoundIncluded[any], models.BoundExcluded[any]] {
	return models.Range[any, models.BoundIncluded[any], models.BoundExcluded[any]]{
		Begin: &models.BoundIncluded[any]{Value: a},
		End:   &models.BoundExcluded[any]{Value: b},
	}
}
