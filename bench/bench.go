package bench

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/romshark/jscan/v2"
	"github.com/tidwall/gjson"
	"github.com/valyala/fastjson"
)

func JscanAny[S []byte | string](
	t *jscan.Tokenizer[S], str S,
) (s any, err error) {
	var decode func(tokens []jscan.Token[S]) (any, []jscan.Token[S], error)
	decode = func(tokens []jscan.Token[S]) (any, []jscan.Token[S], error) {
		switch tokens[0].Type {
		case jscan.TokenTypeNull:
			return nil, tokens[1:], nil
		case jscan.TokenTypeNumber, jscan.TokenTypeInteger:
			var f64 float64
			f64, err = tokens[0].Float64(str)
			return f64, tokens[1:], nil
		case jscan.TokenTypeTrue:
			return true, tokens[1:], nil
		case jscan.TokenTypeFalse:
			return false, tokens[1:], nil
		case jscan.TokenTypeString:
			return string(str[tokens[0].Index+1 : tokens[0].End-1]), tokens[1:], nil
		case jscan.TokenTypeArray:
			l := make([]any, 0, tokens[0].Elements)
			for tokens = tokens[1:]; tokens[0].Type != jscan.TokenTypeArrayEnd; {
				var v any
				var err error
				if v, tokens, err = decode(tokens); err != nil {
					return nil, nil, err
				}
				l = append(l, v)
			}
			return l, tokens[1:], nil
		case jscan.TokenTypeObject:
			if tokens[0].Elements == 0 {
				return map[string]any{}, tokens[2:], nil
			}
			m := make(map[string]any, tokens[0].Elements)
			for tokens = tokens[1:]; tokens[0].Type != jscan.TokenTypeObjectEnd; {
				key := str[tokens[0].Index+1 : tokens[0].End-1]
				var v any
				var err error
				if v, tokens, err = decode(tokens[1:]); err != nil {
					return nil, nil, err
				}
				m[string(key)] = v
			}
			return m, tokens[1:], nil
		}
		return nil, nil, nil
	}

	errk := t.Tokenize(str, func(tokens []jscan.Token[S]) bool {
		s, _, err = decode(tokens)
		return err != nil
	})
	if errk.IsErr() {
		if errk.Code == jscan.ErrorCodeCallback {
			return nil, err
		}
		return nil, errk
	}
	return s, nil
}

func JscanBoolMatrix[S []byte | string](
	t *jscan.Tokenizer[S], str S,
) (s [][]bool, err error) {
	errk := t.Tokenize(str, func(tokens []jscan.Token[S]) bool {
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

func JscanIntSlice[S []byte | string](
	t *jscan.Tokenizer[S], str S,
) (s []int, err error) {
	errk := t.Tokenize(str, func(tokens []jscan.Token[S]) bool {
		if tokens[0].Type != jscan.TokenTypeArray {
			return true
		}
		l := tokens[0].Elements
		s = make([]int, l)
		for i, t := 0, 1; tokens[t].Type != jscan.TokenTypeArrayEnd; i, t = i+1, t+1 {
			if s[i], err = tokens[t].Int(str); err != nil {
				err = fmt.Errorf(
					"at index %d: expected int, received: %s",
					tokens[t].Index, tokens[t].Type.String(),
				)
				return true
			}
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

func JscanStringSlice[S []byte | string](
	t *jscan.Tokenizer[S], str S,
) (s []string, err error) {
	errk := t.Tokenize(str, func(tokens []jscan.Token[S]) bool {
		if tokens[0].Type != jscan.TokenTypeArray {
			return true
		}
		l := tokens[0].Elements
		s = make([]string, l)
		for i, t := 0, 1; tokens[t].Type != jscan.TokenTypeArrayEnd; i, t = i+1, t+1 {
			if s[i], err = tokens[t].String(str); err != nil {
				err = fmt.Errorf(
					"at index %d: expected string, received: %s",
					tokens[t].Index, tokens[t].Type.String(),
				)
				return true
			}
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

func JscanMapStringString[S []byte | string](
	t *jscan.Tokenizer[S], str S,
) (m map[string]string, err error) {
	errk := t.Tokenize(str, func(tokens []jscan.Token[S]) bool {
		if tokens[0].Type != jscan.TokenTypeObject {
			return true
		}
		m = make(map[string]string, tokens[0].Elements)
		for ti := 1; tokens[ti].Type != jscan.TokenTypeObjectEnd; {
			key := str[tokens[ti].Index+1 : tokens[ti].End-1]
			ti++
			if tokens[ti].Type != jscan.TokenTypeString {
				return true
			}
			m[string(key)] = string(str[tokens[ti].Index+1 : tokens[ti].End-1])
			ti++
		}
		return false
	})
	if errk.IsErr() {
		if errk.Code == jscan.ErrorCodeCallback {
			return m, ErrInvalid
		}
		return m, errk
	}
	return m, nil
}

func JscanStruct3[S []byte | string](
	t *jscan.Tokenizer[S], src S,
) (s Struct3, err error) {
	errk := t.Tokenize(src, func(tokens []jscan.Token[S]) bool {
		if tokens[0].Type != jscan.TokenTypeObject {
			return true
		}
		for ti := 1; tokens[ti].Type != jscan.TokenTypeObjectEnd; {
			key := src[tokens[ti].Index+1 : tokens[ti].End-1]
			ti++
			switch string(key) {
			case "name":
				if s.Name, err = tokens[ti].String(src); err != nil {
					return true
				}
				ti++
			case "number":
				if s.Number, err = tokens[ti].Int(src); err != nil {
					return true
				}
				ti++
			case "tags":
				if tokens[ti].Type != jscan.TokenTypeArray {
					err = ErrInvalid
					return true
				}
				s.Tags = make([]string, 0, tokens[ti].Elements)
				for ti = ti + 1; tokens[ti].Type != jscan.TokenTypeArrayEnd; ti++ {
					if tokens[ti].Type != jscan.TokenTypeString {
						err = ErrInvalid
						return true
					}
					val := src[tokens[ti].Index+1 : tokens[ti].End-1]
					s.Tags = append(s.Tags, string(val))
				}
				ti++
			}
		}
		return false
	})
	if errk.IsErr() {
		if errk.Code == jscan.ErrorCodeCallback {
			return s, err
		}
		return s, errk
	}
	return s, nil
}

func GJSONAny(j []byte) (any, error) {
	if !gjson.ValidBytes(j) {
		return nil, errors.New("invalid")
	}
	return gjson.ParseBytes(j).Value(), nil
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
		return nil, ErrInvalid
	}
	l := gjson.ParseBytes(j).Array()
	a := make([]int, 0, len(l))
	for _, item := range l {
		a = append(a, int(item.Int()))
	}
	return a, nil
}

func GJSONArrayString(j []byte) ([]string, error) {
	if !gjson.ValidBytes(j) {
		return nil, ErrInvalid
	}
	l := gjson.ParseBytes(j).Array()
	a := make([]string, 0, len(l))
	for _, item := range l {
		a = append(a, item.String())
	}
	return a, nil
}

func GJSONStruct3(j []byte) (s Struct3, err error) {
	if !gjson.ValidBytes(j) {
		return s, ErrInvalid
	}
	v := gjson.ParseBytes(j)
	if !v.IsObject() {
		return s, ErrInvalid
	}
	v.ForEach(func(key, value gjson.Result) bool {
		switch key.Str {
		case "name":
			if value.Type != gjson.String {
				err = ErrInvalid
				return false
			}
			s.Name = value.String()
		case "number":
			if value.Type != gjson.Number {
				err = ErrInvalid
				return false
			}
			var v int64
			v, err = strconv.ParseInt(value.Raw, 10, 64)
			if err != nil {
				return false
			}
			s.Number = int(v)
		case "tags":
			if !value.IsArray() {
				err = ErrInvalid
				return false
			}
			a := value.Array()
			s.Tags = make([]string, len(a))
			for i := range a {
				if a[i].Type != gjson.String {
					err = ErrInvalid
					return false
				}
				s.Tags[i] = a[i].String()
			}
		default:
			err = ErrInvalid
			return false
		}
		return true
	})
	return s, nil
}

func GJSONMapStringString(j []byte) (map[string]string, error) {
	if !gjson.ValidBytes(j) {
		return nil, errors.New("invalid")
	}
	r := gjson.ParseBytes(j).Map()
	m := make(map[string]string, len(r))
	for key, val := range r {
		m[key] = val.Str
	}
	return m, nil
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

func FastjsonArrayString(j []byte) ([]string, error) {
	v, err := fastjson.ParseBytes(j)
	if err != nil {
		return nil, err
	}
	va := v.GetArray()
	a := make([]string, len(va))
	for i := range va {
		s, err := va[i].StringBytes()
		if err != nil {
			return nil, err
		}
		a[i] = string(s)
	}
	return a, nil
}

func FastjsonStruct3(j []byte) (s Struct3, err error) {
	v, err := fastjson.ParseBytes(j)
	if err != nil {
		return s, err
	}

	o, err := v.Object()
	if err != nil {
		return s, err
	}
	o.Visit(func(key []byte, v *fastjson.Value) {
		switch string(key) {
		case "name":
			if v.Type() != fastjson.TypeString {
				err = ErrInvalid
				return
			}
			v := v.String()
			s.Name = v[1 : len(v)-1]
		case "number":
			if s.Number, err = v.Int(); err != nil {
				return
			}
		case "tags":
			var a []*fastjson.Value
			if a, err = v.Array(); err != nil {
				return
			}
			s.Tags = make([]string, len(a))
			for i := range a {
				if a[i].Type() != fastjson.TypeString {
					err = ErrInvalid
					return
				}
				v := a[i].String()
				s.Tags[i] = v[1 : len(v)-1]
			}
		default:
			err = ErrInvalid
			return
		}
	})
	return s, nil
}

var ErrInvalid = errors.New("invalid")
