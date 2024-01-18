package bench

import (
	"errors"
	"fmt"
	"strconv"
	"unsafe"

	"github.com/romshark/jscan/v2"
	"github.com/tidwall/gjson"
	"github.com/valyala/fastjson"
)

func DecodeArray2DCustomParser[S []byte | string](
	t *jscan.Tokenizer[S], str S,
) (s [][]bool, err error) {
	errk := t.Tokenize(str, func(tokens []jscan.Token) bool {
		if tokens[0].Type != jscan.TokenTypeArray {
			return true
		}
		l := tokens[0].Elements
		s = make([][]bool, l)
		t := 1
		for i1 := 0; i1 < l; i1++ {
			if tokens[t].Type != jscan.TokenTypeArray {
				err = fmt.Errorf(
					"at index %d: expected array, received: %s",
					tokens[t].Index, tokens[t].Type.String(),
				)
				return true
			}
			s[i1] = make([]bool, tokens[t].Elements)
			t++
			for i2 := 0; tokens[t].Type != jscan.TokenTypeArrayEnd; i2++ {
				if tokens[t].Type != jscan.TokenTypeTrue &&
					tokens[t].Type != jscan.TokenTypeFalse {
					err = fmt.Errorf(
						"at index %d: expected boolean, received: %s",
						tokens[t].Index, tokens[t].Type.String(),
					)
					return true
				}
				s[i1][i2] = tokens[t].Type == jscan.TokenTypeTrue
				t++
			}
			t++
		}
		return false
	})
	if errk.IsErr() {
		if errk.Code == jscan.ErrorCodeCallback {
			return nil, err
		}
		return nil, errk
	}
	return s, nil
}

func DecodeArrayIntCustomParser[S []byte | string](
	t *jscan.Tokenizer[S], str S,
) (s []int, err error) {
	var sz S
	atoi := func(s S) (int, error) { return strconv.Atoi(string(s)) }
	if _, ok := any(sz).([]byte); ok {
		atoi = func(s S) (int, error) {
			su := unsafe.String(unsafe.SliceData([]byte(s)), len(s))
			return strconv.Atoi(su)
		}
	}

	errk := t.Tokenize(str, func(tokens []jscan.Token) bool {
		if tokens[0].Type != jscan.TokenTypeArray {
			return true
		}
		l := tokens[0].Elements
		s = make([]int, l)
		for i, t := 0, 1; tokens[t].Type != jscan.TokenTypeArrayEnd; i, t = i+1, t+1 {
			if tokens[t].Type != jscan.TokenTypeInteger {
				err = fmt.Errorf(
					"at index %d: expected int, received: %s",
					tokens[t].Index, tokens[t].Type.String(),
				)
				return true
			}
			var v int
			v, err = atoi(str[tokens[t].Index:tokens[t].End])
			if err != nil {
				return true
			}
			s[i] = v
		}
		return false
	})
	if errk.IsErr() {
		if errk.Code == jscan.ErrorCodeCallback {
			return nil, err
		}
		return nil, errk
	}
	return s, nil
}

func GJSONArrayBool2D(j []byte) ([][]bool, error) {
	if !gjson.ValidBytes(j) {
		return nil, errors.New("invalid")
	}
	l1 := gjson.ParseBytes(j).Array()
	matrix := make([][]bool, 0, len(l1))
	for _, array := range l1 {
		l2 := array.Array()
		a2 := make([]bool, 0, len(l2))
		for _, item := range l2 {
			a2 = append(a2, item.Bool())
		}
		matrix = append(matrix, a2)
	}
	return matrix, nil
}

func GJSONArrayInt(j []byte) ([]int, error) {
	if !gjson.ValidBytes(j) {
		return nil, errors.New("invalid")
	}
	l := gjson.ParseBytes(j).Array()
	a := make([]int, 0, len(l))
	for _, item := range l {
		a = append(a, int(item.Int()))
	}
	return a, nil
}

func FastjsonArrayInt(j []byte) ([]int, error) {
	v, err := fastjson.ParseBytes(j)
	if err != nil {
		return nil, err
	}
	va := v.GetArray()
	a := make([]int, len(va))
	for i := range va {
		a[i] = va[i].GetInt()
	}
	return a, nil
}
