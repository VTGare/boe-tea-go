package slices

func Any[T comparable](ss []T, cmp T) bool {
	for _, s := range ss {
		if s == cmp {
			return true
		}
	}

	return false
}

func AnyFunc[T comparable](ss []T, f func(T) bool) bool {
	for _, s := range ss {
		if f(s) {
			return true
		}
	}

	return false
}

func Filter[T any](slice []T, f func(T) bool) []T {
	filtered := make([]T, 0)

	for _, val := range slice {
		if f(val) {
			filtered = append(filtered, val)
		}
	}

	return filtered
}

func Map[T any](slice []T, f func(int, T) T) []T {
	if len(slice) == 0 {
		return nil
	}

	mapped := make([]T, 0)
	for ind, val := range slice {
		mapped = append(mapped, f(ind, val))
	}

	return mapped
}

func Delete[T any](slice []T, s int) []T {
	return append(slice[:s], slice[s+1:]...)
}
