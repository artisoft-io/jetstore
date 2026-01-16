package utils

import "errors"

// Simple utility to zip 2 slices into a map

// ZipSlices zips two slices of any type into a map.
// K must be a comparable type (e.g., string, int, float, struct with comparable fields).
func ZipSlices[K comparable, V any](keys []K, values []V) (map[K]V, error) {
	if len(keys) != len(values) {
		return nil, errors.New("slices must be of the same length")
	}

	result := make(map[K]V)
	for i, key := range keys {
		result[key] = values[i]
	}

	return result, nil
}

// ZipSlices zips two slices with key of any type, and value as any, into a map. 
// Omit nil values on output map.
// K must be a comparable type (e.g., string, int, float, struct with comparable fields).
func ZipSlicesNoNil[K comparable](keys []K, values []any) (map[K]any, error) {
	if len(keys) != len(values) {
		return nil, errors.New("slices must be of the same length")
	}

	result := make(map[K]any)
	for i, key := range keys {
		if values[i] != nil {
		result[key] = values[i]
		}
	}

	return result, nil
}
