package arrays

func AnyString(ss []string, cmp string) bool {
	for _, s := range ss {
		if s == cmp {
			return true
		}
	}

	return false
}

func AnyStringFunc(ss []string, f func(string) bool) bool {
	for _, s := range ss {
		if f(s) {
			return true
		}
	}

	return false
}

func FilterString(ss []string, f func(string) bool) []string {
	filtered := make([]string, 0)

	for _, str := range ss {
		if f(str) {
			filtered = append(filtered, str)
		}
	}

	return filtered
}

func MapString(ss []string, f func(string) string) []string {
	mapped := make([]string, 0)

	for _, str := range ss {
		mapped = append(mapped, f(str))
	}

	return mapped
}
