package dgoutils

import (
	"errors"
	"strconv"
	"strings"
)

var (
	ErrNotRange    = errors.New("not range")
	ErrRangeSyntax = errors.New("range low is higher than range high")
)

// TrimmerRaw trims <> in case someone wraps the link in it, and characters '!', '@', '#', and '&' for channels and user mentions.
func TrimmerRaw(arg string) string {
	return strings.Trim(arg, "<!@#&>")
}

type Range struct {
	Low  int
	High int
}

func NewRange(s string) (*Range, error) {
	before, after, ok := strings.Cut(s, "-")
	if !ok {
		return nil, ErrNotRange
	}

	lowStr := before
	highStr := after

	low, err := strconv.Atoi(lowStr)
	if err != nil {
		return nil, err
	}

	high, err := strconv.Atoi(highStr)
	if err != nil {
		return nil, err
	}

	if low > high {
		return nil, ErrRangeSyntax
	}

	return &Range{
		Low:  low,
		High: high,
	}, nil
}

func (r *Range) Array() []int {
	arr := make([]int, 0)
	for i := r.Low; i <= r.High; i++ {
		arr = append(arr, i)
	}

	return arr
}

func (r *Range) Map() map[int]struct{} {
	m := make(map[int]struct{})
	for i := r.Low; i <= r.High; i++ {
		m[i] = struct{}{}
	}
	return m
}
