package main

import "math/rand"

// RangeInclusive is a helper type to represent a range of ports.
type RangeInclusive struct {
	min, max uint16
}

// IsEmpty checks if the range is empty.
func (r RangeInclusive) IsEmpty() bool {
	return r.min > r.max
}

// Contains checks if a port is within the range.
func (r RangeInclusive) Contains(port uint16) bool {
	return port >= r.min && port <= r.max
}

// RandomPort returns a random port within the range.
func (r RangeInclusive) RandomPort() uint16 {
	return uint16(rand.Intn(int(r.max-r.min+1))) + r.min
}
