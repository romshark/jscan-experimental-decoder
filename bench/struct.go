package bench

//go:generate easyjson -all struct.go
//go:generate ffjson $GOFILE

type BoolMatrix struct {
	Data [][]bool `json:"data"`
}

type IntArray struct {
	Data []int `json:"data"`
}

type MapStringString struct {
	Data map[string]string `json:"data"`
}

type Struct3 struct {
	Name   string   `json:"name"`
	Number int      `json:"number"`
	Tags   []string `json:"tags"`
}
