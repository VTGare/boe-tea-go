package arrays

import (
	"math/rand"
	"time"
)

func collate[T comparable](a, b T, c ...T) []T {
	slice := make([]T, 0)
	slice = append(slice, a, b)
	return append(slice, c...)
}

func Check[T comparable](compare T, slice []T) bool {
	for _, val := range slice {
		if val == compare {
			return true
		}
	}

	return false
}

func CheckArgs[T comparable](compare, a, b T, c ...T) bool {
	return Check(compare, collate(a, b, c...))
}

func CheckFunc[T comparable](f func(T) bool, slice []T) bool {
	for _, val := range slice {
		if f(val) {
			return true
		}
	}

	return false
}

func CheckFuncArgs[T comparable](f func(T) bool, a, b T, c ...T) bool {
	return CheckFunc(f, collate(a, b, c...))
}

func CheckFuncCompare[T comparable](f func(T, T) bool, compare T, slice []T) bool {
	for _, val := range slice {
		if f(compare, val) {
			return true
		}
	}

	return false
}

func CheckFuncCompareArgs[T comparable](f func(T, T) bool, compare, a, b T, c ...T) bool {
	return CheckFuncCompare(f, compare, collate(a, b, c...))
}

func CheckArrays[T comparable](a, b []T) bool {
	for _, val := range a {
		for _, test := range b {
			if val == test {
				return true
			}
		}
	}

	return false
}

func CheckArraysFunc[T comparable](f func(T, T) bool, a, b []T) bool {
	for _, val := range a {
		for _, test := range b {
			if f(val, test) {
				return true
			}
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

func Find[T any](slice []T, f func(T) bool) T {
	var n T
	if len(slice) == 0 {
		return n
	}

	for _, val := range slice {
		if f(val) {
			return val
		}
	}

	return n
}

func Remove[T comparable](ss []T, match T) []T {
	for i, s := range ss {
		if s == match {
			return append(ss[:i], ss[i+1:]...)
		}
	}
	return ss
}

func RandomElement[T any](slice []T) *T {
	l := len(slice)
	if l == 0 {
		return nil
	}

	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)

	return &slice[r.Intn(l)]
}
