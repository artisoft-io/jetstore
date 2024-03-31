package datatable

import "strings"

func GetLastComponent(path string) (result string) {
	idx := strings.LastIndex(path, "/")
	if idx >= 0 && idx < len(path)-1 {
		return (path)[idx+1:]
	} else {
		return path
	}
}