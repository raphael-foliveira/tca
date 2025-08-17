package utils

func MapMany[T any, K any](orig []T, fn func(T) K) []K {
	result := make([]K, len(orig))
	for i, v := range orig {
		result[i] = fn(v)
	}
	return result
}
