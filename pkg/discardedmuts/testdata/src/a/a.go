package a

// Define a simple struct for testing
type TestStruct struct {
	Name         string
	collectables []Collectable
}

type Collectable struct {
	checked bool
}

func something(ts TestStruct) {
	ts.Name = "Fred" // want "modification to value parameter ts.Name will be discarded"
}

func somethingElse(ts *TestStruct) {
	for _, c := range ts.collectables {
		c.checked = true // want "modification to element of slice c.checked will be discarded"
	}
}

func modifySlice(ts *TestStruct) {
	for _, c := range ts.collectables {
		SetChecked(&c, true) // want "passing address of slice element c will modify a copy, not the original"
	}
}

func SetChecked(c *Collectable, value bool) {
	c.checked = value
}

func modifyArraySlice() [3]int {
	arr := [3]int{1, 2, 3}
	for _, v := range arr {
		modifyElement(&v, 0)
	}
	return arr
}

func modifyElement(v *int, n int) {
	*v = n
}
