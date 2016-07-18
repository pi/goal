package set

//
// StringSet - set of strings. Defined as map[string]bool
//
type StringSet map[string]bool

// Strings constructs set from slice of strings
func Strings(values []string) StringSet {
	s := make(StringSet)
	for _, str := range values {
		s[str] = true
	}
	return s
}

// OfStrings constructs set of string arguments
func OfStrings(strings ...string) (s StringSet) {
	return Strings(strings)
}

// Len returns number of elements in the receiver
func (s StringSet) Len() int {
	return len(s)
}

// AsSlice return values of the reciver as slice of strings
func (s StringSet) AsSlice() []string {
	result := make([]string, len(s), len(s))
	i := 0
	for v := range s {
		result[i] = v
		i++
	}
	return result
}

// Includes returns true if the receiver contains value
func (s StringSet) Includes(value string) bool {
	if v, ok := s[value]; ok {
		return v
	}
	return false
}

// IncludesAny returns true if the receiver contains any value from values
func (s StringSet) IncludesAny(values []string) bool {
	for _, v := range values {
		if _, ok := s[v]; ok {
			return true
		}
	}
	return false
}

// Intersects returns true if the receiver contains any value from other set
func (s StringSet) Intersects(other StringSet) bool {
	for v := range other {
		if _, ok := s[v]; ok {
			return true
		}
	}
	return false
}

// Intersect returns set of common to receiver and the other set values
func (s StringSet) Intersect(other StringSet) StringSet {
	result := make(StringSet)
	for v := range s {
		if _, ok := other[v]; ok {
			result.Add(v)
		}
	}
	return result
}

// Union returns set of all values of the receiver and other set
func (s StringSet) Union(other StringSet) StringSet {
	result := make(StringSet)
	for v := range s {
		result[v] = true
	}
	for v := range other {
		result[v] = true
	}
	return result
}

// Clone returns copy of the receiver
func (s StringSet) Clone() StringSet {
	result := make(StringSet)
	for v := range s {
		result[v] = true
	}
	return result
}

// Add adds element to the receiver
func (s StringSet) Add(str string) {
	s[str] = true
}

// Remove deletes element from the receiver
func (s StringSet) Remove(str string) {
	delete(s, str)
}

// AddAll adds all values from slice of strings
func (s StringSet) AddAll(values []string) {
	for _, str := range values {
		s[str] = true
	}
}

// AddSet adds all elements of src set to the receiver
func (s StringSet) AddSet(src StringSet) {
	for str := range src {
		s[str] = true
	}
}

// RemoveAll deletes all values in slice of strings from the receiver
func (s StringSet) RemoveAll(values []string) {
	for _, str := range values {
		delete(s, str)
	}
}

//
// IntSet is a set of integers. Defined as map[int]bool
//
type IntSet map[int]bool

// Ints constructs IntSet from slice of integers
func Ints(values []int) IntSet {
	s := make(IntSet)
	for _, i := range values {
		s[i] = true
	}
	return s
}
func OfInts(ints ...int) (s IntSet) {
	return Ints(ints)
}

// Len returns number of elements in the receiver
func (s IntSet) Len() int {
	return len(s)
}

// AsSlice returns the receiver's elements as slice of ints
func (s IntSet) AsSlice() []int {
	result := make([]int, len(s), len(s))
	i := 0
	for v := range s {
		result[i] = v
		i++
	}
	return result
}

func (s IntSet) Includes(v int) bool {
	_, ok := s[v]
	return ok
}
func (s IntSet) IncludesAny(values []int) bool {
	for _, v := range values {
		if _, ok := s[v]; ok {
			return true
		}
	}
	return false
}
func (s IntSet) Intersects(other IntSet) bool {
	for v := range other {
		if _, ok := s[v]; ok {
			return true
		}
	}
	return false
}
func (s IntSet) Intersect(other IntSet) IntSet {
	result := make(IntSet)
	for v := range s {
		if _, ok := other[v]; ok {
			result.Add(v)
		}
	}
	return result
}
func (s IntSet) Union(other IntSet) IntSet {
	result := make(IntSet)
	for v := range s {
		result[v] = true
	}
	for v := range other {
		result[v] = true
	}
	return result
}
func (s IntSet) Add(v int) {
	s[v] = true
}
func (s IntSet) Remove(v int) {
	delete(s, v)
}
func (s IntSet) AddAll(values []int) {
	for _, v := range values {
		s[v] = true
	}
}
func (s IntSet) AddSet(src IntSet) {
	for v := range src {
		s[v] = true
	}
}
func (s IntSet) RemoveAll(values []int) {
	for _, v := range values {
		delete(s, v)
	}
}
