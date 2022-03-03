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
	if len(ss) == 0 {
		return nil
	}

	mapped := make([]string, 0)

	for _, str := range ss {
		mapped = append(mapped, f(str))
	}

	return mapped
}

func FilterInt(slice []int, f func(int) bool) []int {
	filtered := make([]int, 0)

	for _, num := range slice {
		if f(num) {
			filtered = append(filtered, num)
		}
	}

	return filtered
}

func MapInt(slice []int, f func(int) int) []int {
	if len(slice) == 0 {
		return nil
	}

	mapped := make([]int, 0)
	for _, num := range slice {
		mapped = append(mapped, f(num))
	}

	return mapped
}

func RemoveInt(slice []int, s int) []int {
	return append(slice[:s], slice[s+1:]...)
}
