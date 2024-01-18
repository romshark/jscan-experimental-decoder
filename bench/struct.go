package bench

//go:generate easyjson -all struct.go
//go:generate ffjson $GOFILE

type BoolMatrix struct {
	Data [][]bool `json:"data"`
}

type IntArray struct {
	Data []int `json:"data"`
}
