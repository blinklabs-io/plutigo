package machine

func indexExists[T any](arr []T, index int) bool {
	return index >= 0 && index < len(arr)
}
