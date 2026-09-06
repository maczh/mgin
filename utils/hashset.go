package utils

type HashSet struct {
	set map[string]bool
}

func NewHashSet() *HashSet {
	return &HashSet{make(map[string]bool)}
}

func (set *HashSet) Add(i string) bool {
	if set == nil {
		return false
	}
	if set.set == nil {
		set.set = make(map[string]bool)
	}
	_, found := set.set[i]
	set.set[i] = true
	return !found //False if it existed already
}

func (set *HashSet) Exists(i string) bool {
	if set == nil || set.set == nil {
		return false
	}
	_, found := set.set[i]
	return found //true if it existed already
}

func (set *HashSet) Remove(i string) {
	if set == nil || set.set == nil {
		return
	}
	delete(set.set, i)
}

func (set *HashSet) Members() []string {
	if set == nil || set.set == nil {
		return nil
	}
	members := make([]string, 0, len(set.set))
	for k := range set.set {
		members = append(members, k)
	}
	return members
}
