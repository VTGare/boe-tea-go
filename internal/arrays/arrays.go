package arrays

import (
	"math/rand/v2"
	"slices"
)

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

func Find[T any](slice []T, f func(T) bool) T {
	var n T
	if i := slices.IndexFunc(slice, f); i >= 0 {
		return slice[i]
	}

	return n
}

func Remove[T comparable](ss []T, match T) []T {
	i := slices.Index(ss, match)
	if i < 0 {
		return ss
	}
	return slices.Delete(ss, i, i+1)
}

func RandomElement[T any](slice []T) *T {
	if len(slice) == 0 {
		return nil
	}

	return &slice[rand.IntN(len(slice))]
}
