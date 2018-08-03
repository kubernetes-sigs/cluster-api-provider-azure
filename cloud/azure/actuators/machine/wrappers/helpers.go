package wrappers

func contains(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

func index(slice []string, str string) int {
	for i, item := range slice {
		if item == str {
			return i
		}
	}
	return -1
}

func remove(slice []string, str string) []string {
	i := index(slice, str)
	if i == -1 {
		return slice
	}
	slice[len(slice)-1], slice[i] = slice[i], slice[len(slice)-1]
	return slice[:len(slice)-1]
}
