/*
Package util contains various utilities for use by the lantern system.
*/
package util

/*
StringSet is a set of strings backed by a map of strings.
*/
type StringSet struct {
	m map[string]bool
}

// Add() adds the value to the set and returns true if it didn't exist
// previously.
func (set *StringSet) Add(val string) bool {
	_, found := set.m[val]
	set.m[val] = true
	return !found
}

// Remove() removes the value from the set.
func (set *StringSet) Remove(val string) {
	delete(set.m, val)
}

// Contains() checks if the set contains the given value.
func (set *StringSet) Contains(val string) (found bool) {
	_, found = set.m[val]
	return
}
