package util

type StringSet struct {
	m map[string]bool
}

func (set *StringSet) Add(val string) bool {
	_, found := set.m[val]
	set.m[val] = true
	return !found
}

func (set *StringSet) Remove(val string) {
	delete(set.m, val)
}

func (set *StringSet) Contains(val string) (found bool) {
	_, found = set.m[val]
	return
}
