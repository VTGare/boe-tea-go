package arrays

func Any[T comparable](slice []T, cmp T) bool {
	for _, val := range slice {
		if val == cmp {
			return true
		}
	}

	return false
}

func AnyFunc[T comparable](slice []T, f func(T) bool) bool {
	for _, val := range slice {
		if f(val) {
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

func Map[T any](slice []T, f func(T) T) []T {
	if len(slice) == 0 {
		return nil
	}

	mapped := make([]T, 0)

	for _, val := range slice {
		mapped = append(mapped, f(val))
	}

	return mapped
}
