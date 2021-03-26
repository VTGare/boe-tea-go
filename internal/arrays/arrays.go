package arrays

func AnyString(ss []string, cmp string) bool {
	for _, s := range ss {
		if s == cmp {
			return true
		}
	}

	return false
}

func AnyStringFunc(ss []string, f func(s string) bool) bool {
	for _, s := range ss {
		if f(s) {
			return true
		}
	}

	return false
}
