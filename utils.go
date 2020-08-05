package main

type stringSlice []string

func (s stringSlice) has(value string) bool {
	for _, v := range s {
		if v == value {
			return true
		}
	}
	return false
}
