package ffjsongen

//go:generate ffjson $GOFILE

type Any struct {
	Data any `json:"data"`
}

type PtrInt struct {
	Data *int `json:"data"`
}

type BoolMatrix struct {
	Data [][]bool `json:"data"`
}

type IntArray struct {
	Data []int `json:"data"`
}

type StringArray struct {
	Data []string `json:"data"`
}

type MapStringString struct {
	Data map[string]string `json:"data"`
}

type MapIntMapStringStruct3 struct {
	// Can't use Struct3 because it fails the code generator
	Data map[int]map[string]struct {
		Name   string   `json:"name"`
		Number int      `json:"number"`
		Tags   []string `json:"tags"`
	} `json:"data"`
}

type Struct3 struct {
	Name   string   `json:"name"`
	Number int      `json:"number"`
	Tags   []string `json:"tags"`
}

type StructVector3D struct{ X, Y, Z float64 }
