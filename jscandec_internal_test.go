package jscandec

import (
	_ "embed"
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppendTypeToStack(t *testing.T) {
	type S1 struct {
		Foo int    `json:"foo"`
		Bar string `json:"bar"`
	}
	type S2 struct {
		S1
		Bar     []string
		Ignored int16 `json:"-"`
		Bazz    int   `json:"bazz"`
	}
	type S3 struct {
		Any   any
		Map   map[string]any
		Slice []any
	}
	type S4 struct {
		Name        string                  `json:"name"`
		Unmarshaler testImplJSONUnmarshaler `json:"unmar"`
		Tail        []int                   `json:"tail"`
	}
	type S5 struct {
		Name        string                   `json:"name"`
		Unmarshaler *testImplJSONUnmarshaler `json:"unmar"`
		Tail        []int                    `json:"tail"`
	}
	type SStringString struct {
		String string `json:",string"`
	}
	type SStringBool struct {
		Bool bool `json:",string"`
	}
	type SStringFloat32 struct {
		Float32 float32 `json:",string"`
	}
	type SStringFloat64 struct {
		Float64 float64 `json:",string"`
	}
	type SStringInt struct {
		Int int `json:",string"`
	}
	type SStringInt8 struct {
		Int8 int8 `json:",string"`
	}
	type SStringInt16 struct {
		Int16 int16 `json:",string"`
	}
	type SStringInt32 struct {
		Int32 int32 `json:",string"`
	}
	type SStringInt64 struct {
		Int64 int64 `json:",string"`
	}
	type SStringUint struct {
		Uint uint `json:",string"`
	}
	type SStringUint8 struct {
		Uint8 uint8 `json:",string"`
	}
	type SStringUint16 struct {
		Uint16 uint16 `json:",string"`
	}
	type SStringUint32 struct {
		Uint32 uint32 `json:",string"`
	}
	type SStringUint64 struct {
		Uint64 uint64 `json:",string"`
	}
	type SRecurSlice struct {
		ID        string
		Recursion []SRecurSlice
	}
	type SRecurMap struct {
		ID        string
		Recursion map[string]SRecurMap
	}
	type SRecurPtr struct {
		ID        string
		Recursion *SRecurPtr
	}

	tpS3 := reflect.TypeOf(S3{})
	tpS4 := reflect.TypeOf(S4{})
	tpS5 := reflect.TypeOf(S5{})
	tpEmptyIface := reflect.TypeOf(struct{ typ, data uintptr }{})

	for _, td := range []struct {
		Input       any
		ExpectStack []stackFrame[string]
	}{
		{
			Input: string(""),
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeStr,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: int(0),
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeInt,
					Typ:              getTyp(reflect.TypeOf(int(0))),
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: struct{}{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeEmptyStruct,
					Typ:              getTyp(reflect.TypeOf(struct{}{})),
					Size:             reflect.TypeOf(struct{}{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: S1{},
			ExpectStack: []stackFrame[string]{
				{ // S1
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(S1{})),
					RType: reflect.TypeOf(S1{}),
					Size:  reflect.TypeOf(S1{}).Size(),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "foo"},
						{FrameIndex: 2, Name: "bar"},
					},
					ParentFrameIndex: noParentFrame,
				},
				{ // S1.Foo
					Type:             ExpectTypeInt,
					Typ:              getTyp(reflect.TypeOf(int(0))),
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // S1.Bar
					Type:             ExpectTypeStr,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 0,
					Offset:           reflect.TypeOf(S1{}).Field(1).Offset,
				},
			},
		},
		{
			Input: S2{},
			ExpectStack: []stackFrame[string]{
				{ // S2
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(S2{})),
					RType: reflect.TypeOf(S2{}),
					Size:  reflect.TypeOf(S2{}).Size(),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "S1"},
						{FrameIndex: 4, Name: "Bar"},
						{FrameIndex: 5, Name: "bazz"},
					},
					ParentFrameIndex: noParentFrame,
				},
				{ // S2.S1
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(S1{})),
					RType: reflect.TypeOf(S1{}),
					Size:  reflect.TypeOf(S1{}).Size(),
					Fields: []fieldStackFrame{
						{FrameIndex: 2, Name: "foo"},
						{FrameIndex: 3, Name: "bar"},
					},
					ParentFrameIndex: 0,
				},
				{ // S2.S1.Foo
					Type:             ExpectTypeInt,
					Typ:              getTyp(reflect.TypeOf(int(0))),
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 1,
				},
				{ // S2.S1.Bar
					Type:             ExpectTypeStr,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 1,
					Offset:           reflect.TypeOf(S1{}).Field(1).Offset,
				},
				{ // S2.Bar
					Type:             ExpectTypeSliceString,
					Typ:              getTyp(reflect.TypeOf([]string(nil))),
					Size:             reflect.TypeOf([]string(nil)).Size(),
					ParentFrameIndex: 0,
					Offset:           reflect.TypeOf(S2{}).Field(1).Offset,
				},
				{ // S2.Bazz
					Type:             ExpectTypeInt,
					Typ:              getTyp(reflect.TypeOf(int(0))),
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 0,
					Offset:           reflect.TypeOf(S2{}).Field(3).Offset,
				},
			},
		},
		{
			Input: []bool{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSliceBool,
					Typ:              getTyp(reflect.TypeOf([]bool(nil))),
					Size:             reflect.TypeOf([]bool(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: []string{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSliceString,
					Typ:              getTyp(reflect.TypeOf([]string(nil))),
					Size:             reflect.TypeOf([]string(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: []int{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSliceInt,
					Typ:              getTyp(reflect.TypeOf([]int(nil))),
					Size:             reflect.TypeOf([]int(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: []int8{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSliceInt8,
					Typ:              getTyp(reflect.TypeOf([]int8(nil))),
					Size:             reflect.TypeOf([]int8(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: []int16{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSliceInt16,
					Typ:              getTyp(reflect.TypeOf([]int16(nil))),
					Size:             reflect.TypeOf([]int16(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: []int32{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSliceInt32,
					Typ:              getTyp(reflect.TypeOf([]int32(nil))),
					Size:             reflect.TypeOf([]int32(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: []int64{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSliceInt64,
					Typ:              getTyp(reflect.TypeOf([]int64(nil))),
					Size:             reflect.TypeOf([]int64(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: []uint{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSliceUint,
					Typ:              getTyp(reflect.TypeOf([]uint(nil))),
					Size:             reflect.TypeOf([]uint(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: []uint8{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSliceUint8,
					Typ:              getTyp(reflect.TypeOf([]uint8(nil))),
					Size:             reflect.TypeOf([]uint8(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: []uint16{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSliceUint16,
					Typ:              getTyp(reflect.TypeOf([]uint16(nil))),
					Size:             reflect.TypeOf([]uint16(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: []uint32{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSliceUint32,
					Typ:              getTyp(reflect.TypeOf([]uint32(nil))),
					Size:             reflect.TypeOf([]uint32(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: []uint64{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSliceUint64,
					Typ:              getTyp(reflect.TypeOf([]uint64(nil))),
					Size:             reflect.TypeOf([]uint64(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: []byte{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSliceUint8,
					Typ:              getTyp(reflect.TypeOf([]byte(nil))),
					Size:             reflect.TypeOf([]byte(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: [][][]string{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSlice,
					Typ:              getTyp(reflect.TypeOf([][][]string(nil))),
					Size:             reflect.TypeOf([][][]string(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeSlice,
					Typ:              getTyp(reflect.TypeOf([][]string(nil))),
					Size:             reflect.TypeOf([][]string(nil)).Size(),
					ParentFrameIndex: 0,
				},
				{
					Type:             ExpectTypeSliceString,
					Typ:              getTyp(reflect.TypeOf([]string(nil))),
					Size:             reflect.TypeOf([]string(nil)).Size(),
					ParentFrameIndex: 1,
				},
			},
		},
		{
			Input: []S1{},
			ExpectStack: []stackFrame[string]{
				{ // []
					Type:             ExpectTypeSlice,
					Typ:              getTyp(reflect.TypeOf([]S1(nil))),
					Size:             reflect.TypeOf([]S1(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{ // []S1
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(S1{})),
					RType: reflect.TypeOf(S1{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 2, Name: "foo"},
						{FrameIndex: 3, Name: "bar"},
					},
					Size:             reflect.TypeOf(S1{}).Size(),
					ParentFrameIndex: 0,
				},
				{ // []S1.Foo
					Type:             ExpectTypeInt,
					Typ:              getTyp(reflect.TypeOf(int(0))),
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 1,
				},
				{ // []S1.Bar
					Type:             ExpectTypeStr,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 1,
					Offset:           8,
				},
			},
		},
		{
			Input: [0]int{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeArrayLen0,
					Typ:              getTyp(reflect.TypeOf([0]int{})),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: [0][0]string{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeArrayLen0,
					Typ:              getTyp(reflect.TypeOf([0][0]string{})),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: [3][0]string{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeArray,
					Typ:              getTyp(reflect.TypeOf([3][0]string{})),
					Size:             reflect.TypeOf([3][0]string{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeArrayLen0,
					Typ:              getTyp(reflect.TypeOf([0]string{})),
					Cap:              3,
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: [4]int{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeArray,
					Typ:              getTyp(reflect.TypeOf([4]int{})),
					Size:             reflect.TypeOf([4]int{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeInt,
					Typ:              getTyp(reflect.TypeOf(int(0))),
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 0,
					Cap:              4,
				},
			},
		},
		{
			Input: map[string]string{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeMapStringString,
					Typ:              getTyp(reflect.TypeOf(map[string]string(nil))),
					RType:            reflect.TypeOf(map[string]string(nil)),
					Size:             reflect.TypeOf(map[string]string(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: map[string]struct{ F [256]byte }{},
			ExpectStack: []stackFrame[string]{
				{
					Type: ExpectTypeMap,
					Typ:  getTyp(reflect.TypeOf(map[string]struct{ F [256]byte }(nil))),
					RType: reflect.TypeOf(
						map[string]struct{ F [256]byte }(nil),
					),
					Size: reflect.TypeOf(
						map[string]struct{ F [256]byte }(nil),
					).Size(),
					MapValueType: getTyp(
						reflect.TypeOf(struct{ F [256]byte }{}),
					),
					MapCanUseAssignFaststr: false, // Value too big
					ParentFrameIndex:       noParentFrame,
				},
				{ // Key frame
					Type:             ExpectTypeStr,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value frame
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(struct{ F [256]byte }{})),
					RType: reflect.TypeOf(struct{ F [256]byte }{}),
					Size:  reflect.TypeOf(struct{ F [256]byte }{}).Size(),
					Fields: []fieldStackFrame{
						{FrameIndex: 3, Name: "F"},
					},
					ParentFrameIndex: 0,
				},
				{ // Value frame
					Type:             ExpectTypeArray,
					Typ:              getTyp(reflect.TypeOf([256]byte{})),
					Size:             reflect.TypeOf([256]byte{}).Size(),
					ParentFrameIndex: 2,
				},
				{ // Value frame
					Type:             ExpectTypeUint8,
					Typ:              getTyp(reflect.TypeOf(byte(0))),
					Size:             reflect.TypeOf(byte(0)).Size(),
					Cap:              256,
					ParentFrameIndex: 3,
				},
			},
		},
		{
			Input: map[testImplTextUnmarshaler]int{},
			ExpectStack: []stackFrame[string]{
				{
					Type: ExpectTypeMap,
					Typ: getTyp(
						reflect.TypeOf(map[testImplTextUnmarshaler]string(nil)),
					),
					RType: reflect.TypeOf(
						map[testImplTextUnmarshaler]int(nil),
					),
					Size: reflect.TypeOf(
						map[testImplTextUnmarshaler]int(nil),
					).Size(),
					MapValueType:           getTyp(reflect.TypeOf(string(""))),
					MapCanUseAssignFaststr: false, // Key not a string
					ParentFrameIndex:       noParentFrame,
				},
				{ // Key
					Type:             ExpectTypeTextUnmarshaler,
					Typ:              getTyp(reflect.TypeOf(testImplTextUnmarshaler{})),
					RType:            reflect.TypeOf(testImplTextUnmarshaler{}),
					Size:             reflect.TypeOf(testImplTextUnmarshaler{}).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeInt,
					Typ:              getTyp(reflect.TypeOf(int(0))),
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[int]S1{},
			ExpectStack: []stackFrame[string]{
				{
					Type:                   ExpectTypeMap,
					Typ:                    getTyp(reflect.TypeOf(map[int]S1(nil))),
					RType:                  reflect.TypeOf(map[int]S1(nil)),
					Size:                   reflect.TypeOf(map[int]S1(nil)).Size(),
					MapValueType:           getTyp(reflect.TypeOf(S1{})),
					MapCanUseAssignFaststr: false, // Non-string key type
					ParentFrameIndex:       noParentFrame,
				},
				{ // Key frame
					Type:             ExpectTypeInt,
					Typ:              getTyp(reflect.TypeOf(int(0))),
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // S1
					Type: ExpectTypeStruct,
					Typ:  getTyp(reflect.TypeOf(S1{})),
					Size: reflect.TypeOf(S1{}).Size(),
					Fields: []fieldStackFrame{
						{FrameIndex: 3, Name: "foo"},
						{FrameIndex: 4, Name: "bar"},
					},
					RType:            reflect.TypeOf(S1{}),
					ParentFrameIndex: 0,
				},
				{ // S1.Foo
					Type:             ExpectTypeInt,
					Typ:              getTyp(reflect.TypeOf(int(0))),
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 2,
				},
				{ // S1.Bar
					Type:             ExpectTypeStr,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 2,
					Offset:           reflect.TypeOf(S1{}).Field(1).Offset,
				},
			},
		},
		{
			Input: map[int]map[string]float32{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeMap,
					RType: reflect.TypeOf(map[int]map[string]float32(nil)),
					Typ: getTyp(
						reflect.TypeOf(map[int]map[string]float32(nil)),
					),
					MapValueType: getTyp(
						reflect.TypeOf(map[string]float32(nil)),
					),
					MapCanUseAssignFaststr: false, // Non-string key type
					Size: reflect.TypeOf(
						map[int]map[string]float32(nil),
					).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeInt,
					Typ:              getTyp(reflect.TypeOf(int(0))),
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 0,
				},
				{
					Type:  ExpectTypeMap,
					RType: reflect.TypeOf(map[string]float32(nil)),
					Typ: getTyp(reflect.TypeOf(
						map[string]float32(nil)),
					),
					MapValueType:           getTyp(reflect.TypeOf(float32(0))),
					MapCanUseAssignFaststr: true, // Size of float32 is under 128 bytes.
					Size: reflect.TypeOf(
						map[string]float32(nil),
					).Size(),
					ParentFrameIndex: 0,
				},
				{
					Type:             ExpectTypeStr,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 2,
				},
				{
					Type:             ExpectTypeFloat32,
					Typ:              getTyp(reflect.TypeOf(float32(0))),
					Size:             reflect.TypeOf(float32(0)).Size(),
					ParentFrameIndex: 2,
				},
			},
		},
		{
			Input: map[int8]int8{},
			ExpectStack: []stackFrame[string]{
				{
					Type:                   ExpectTypeMap,
					RType:                  reflect.TypeOf(map[int8]int8(nil)),
					Typ:                    getTyp(reflect.TypeOf(map[int8]int8(nil))),
					MapValueType:           getTyp(reflect.TypeOf(int8(0))),
					MapCanUseAssignFaststr: false,
					Size:                   reflect.TypeOf(map[int8]int8(nil)).Size(),
					ParentFrameIndex:       noParentFrame,
				},
				{ // Key
					Type:             ExpectTypeInt8,
					Typ:              getTyp(reflect.TypeOf(int8(0))),
					Size:             reflect.TypeOf(int8(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeInt8,
					Typ:              getTyp(reflect.TypeOf(int8(0))),
					Size:             reflect.TypeOf(int8(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[int16]int16{},
			ExpectStack: []stackFrame[string]{
				{
					Type:                   ExpectTypeMap,
					RType:                  reflect.TypeOf(map[int16]int16(nil)),
					Typ:                    getTyp(reflect.TypeOf(map[int16]int16(nil))),
					MapValueType:           getTyp(reflect.TypeOf(int16(0))),
					MapCanUseAssignFaststr: false,
					Size:                   reflect.TypeOf(map[int16]int16(nil)).Size(),
					ParentFrameIndex:       noParentFrame,
				},
				{ // Key
					Type:             ExpectTypeInt16,
					Typ:              getTyp(reflect.TypeOf(int16(0))),
					Size:             reflect.TypeOf(int16(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeInt16,
					Typ:              getTyp(reflect.TypeOf(int16(0))),
					Size:             reflect.TypeOf(int16(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[int32]int32{},
			ExpectStack: []stackFrame[string]{
				{
					Type:                   ExpectTypeMap,
					RType:                  reflect.TypeOf(map[int32]int32(nil)),
					Typ:                    getTyp(reflect.TypeOf(map[int32]int32(nil))),
					MapValueType:           getTyp(reflect.TypeOf(int32(0))),
					MapCanUseAssignFaststr: false,
					Size:                   reflect.TypeOf(map[int32]int32(nil)).Size(),
					ParentFrameIndex:       noParentFrame,
				},
				{ // Key
					Type:             ExpectTypeInt32,
					Typ:              getTyp(reflect.TypeOf(int32(0))),
					Size:             reflect.TypeOf(int32(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeInt32,
					Typ:              getTyp(reflect.TypeOf(int32(0))),
					Size:             reflect.TypeOf(int32(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[int64]int64{},
			ExpectStack: []stackFrame[string]{
				{
					Type:                   ExpectTypeMap,
					RType:                  reflect.TypeOf(map[int64]int64(nil)),
					Typ:                    getTyp(reflect.TypeOf(map[int64]int64(nil))),
					MapValueType:           getTyp(reflect.TypeOf(int64(0))),
					MapCanUseAssignFaststr: false,
					Size:                   reflect.TypeOf(map[int64]int64(nil)).Size(),
					ParentFrameIndex:       noParentFrame,
				},
				{ // Key
					Type:             ExpectTypeInt64,
					Typ:              getTyp(reflect.TypeOf(int64(0))),
					Size:             reflect.TypeOf(int64(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeInt64,
					Typ:              getTyp(reflect.TypeOf(int64(0))),
					Size:             reflect.TypeOf(int64(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[uint]uint{},
			ExpectStack: []stackFrame[string]{
				{
					Type:                   ExpectTypeMap,
					RType:                  reflect.TypeOf(map[uint]uint(nil)),
					Typ:                    getTyp(reflect.TypeOf(map[uint]uint(nil))),
					MapValueType:           getTyp(reflect.TypeOf(uint(0))),
					MapCanUseAssignFaststr: false,
					Size:                   reflect.TypeOf(map[uint]uint(nil)).Size(),
					ParentFrameIndex:       noParentFrame,
				},
				{ // Key
					Type:             ExpectTypeUint,
					Typ:              getTyp(reflect.TypeOf(uint(0))),
					Size:             reflect.TypeOf(uint(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeUint,
					Typ:              getTyp(reflect.TypeOf(uint(0))),
					Size:             reflect.TypeOf(uint(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[uint8]uint8{},
			ExpectStack: []stackFrame[string]{
				{
					Type:                   ExpectTypeMap,
					RType:                  reflect.TypeOf(map[uint8]uint8(nil)),
					Typ:                    getTyp(reflect.TypeOf(map[uint8]uint8(nil))),
					MapValueType:           getTyp(reflect.TypeOf(uint8(0))),
					MapCanUseAssignFaststr: false,
					Size:                   reflect.TypeOf(map[uint8]uint8(nil)).Size(),
					ParentFrameIndex:       noParentFrame,
				},
				{ // Key
					Type:             ExpectTypeUint8,
					Typ:              getTyp(reflect.TypeOf(uint8(0))),
					Size:             reflect.TypeOf(uint8(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeUint8,
					Typ:              getTyp(reflect.TypeOf(uint8(0))),
					Size:             reflect.TypeOf(uint8(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[uint16]uint16{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeMap,
					RType: reflect.TypeOf(map[uint16]uint16(nil)),
					Typ: getTyp(reflect.TypeOf(
						map[uint16]uint16(nil)),
					),
					MapValueType:           getTyp(reflect.TypeOf(uint16(0))),
					MapCanUseAssignFaststr: false,
					Size:                   reflect.TypeOf(map[uint16]uint16(nil)).Size(),
					ParentFrameIndex:       noParentFrame,
				},
				{ // Key
					Type:             ExpectTypeUint16,
					Typ:              getTyp(reflect.TypeOf(uint16(0))),
					Size:             reflect.TypeOf(uint16(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeUint16,
					Typ:              getTyp(reflect.TypeOf(uint16(0))),
					Size:             reflect.TypeOf(uint16(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[uint32]uint32{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeMap,
					RType: reflect.TypeOf(map[uint32]uint32(nil)),
					Typ: getTyp(reflect.TypeOf(
						map[uint32]uint32(nil)),
					),
					MapValueType:           getTyp(reflect.TypeOf(uint32(0))),
					MapCanUseAssignFaststr: false,
					Size:                   reflect.TypeOf(map[uint32]uint32(nil)).Size(),
					ParentFrameIndex:       noParentFrame,
				},
				{ // Key
					Type:             ExpectTypeUint32,
					Typ:              getTyp(reflect.TypeOf(uint32(0))),
					Size:             reflect.TypeOf(uint32(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeUint32,
					Typ:              getTyp(reflect.TypeOf(uint32(0))),
					Size:             reflect.TypeOf(uint32(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[uint64]uint64{},
			ExpectStack: []stackFrame[string]{
				{
					Type: ExpectTypeMap,
					Typ: getTyp(reflect.TypeOf(
						map[uint64]uint64(nil),
					)),
					RType:                  reflect.TypeOf(map[uint64]uint64(nil)),
					MapValueType:           getTyp(reflect.TypeOf(uint64(0))),
					MapCanUseAssignFaststr: false,
					Size:                   reflect.TypeOf(map[uint64]uint64(nil)).Size(),
					ParentFrameIndex:       noParentFrame,
				},
				{ // Key
					Type:             ExpectTypeUint64,
					Typ:              getTyp(reflect.TypeOf(uint64(0))),
					Size:             reflect.TypeOf(uint64(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeUint64,
					Typ:              getTyp(reflect.TypeOf(uint64(0))),
					Size:             reflect.TypeOf(uint64(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: S3{},
			ExpectStack: []stackFrame[string]{
				{ // S3
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(S3{})),
					RType: reflect.TypeOf(S3{}),
					Size:  reflect.TypeOf(S3{}).Size(),
					Fields: []fieldStackFrame{
						{Name: "Any", FrameIndex: 1},
						{Name: "Map", FrameIndex: 2},
						{Name: "Slice", FrameIndex: 5},
					},
					ParentFrameIndex: noParentFrame,
				},
				{ // S3.Any
					Type:             ExpectTypeAny,
					Typ:              getTyp(tpEmptyIface),
					Size:             tpEmptyIface.Size(),
					ParentFrameIndex: 0,
					Offset:           tpS3.Field(0).Offset,
				},
				{ // S3.Map
					Type:                   ExpectTypeMap,
					Typ:                    getTyp(reflect.TypeOf(map[string]any(nil))),
					Size:                   reflect.TypeOf(map[string]any(nil)).Size(),
					RType:                  reflect.TypeOf(map[string]any(nil)),
					MapValueType:           getTyp(reflect.TypeOf(any(0))),
					MapCanUseAssignFaststr: true,
					ParentFrameIndex:       0,
					Offset:                 tpS3.Field(1).Offset,
				},
				{ // Key frame
					Type:             ExpectTypeStr,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 2,
				},
				{ // Value frame
					Type:             ExpectTypeAny,
					Typ:              getTyp(tpEmptyIface),
					Size:             tpEmptyIface.Size(),
					ParentFrameIndex: 2,
				},
				{ // S3.Slice
					Type:             ExpectTypeSlice,
					Typ:              getTyp(reflect.TypeOf([]any(nil))),
					Size:             reflect.TypeOf([]any(nil)).Size(),
					ParentFrameIndex: 0,
					Offset:           tpS3.Field(2).Offset,
				},
				{ // S3.Slice
					Type:             ExpectTypeAny,
					Typ:              getTyp(tpEmptyIface),
					Size:             tpEmptyIface.Size(),
					ParentFrameIndex: 5,
				},
			},
		},
		{
			Input: Ptr(int(0)),
			ExpectStack: []stackFrame[string]{
				{ // *
					Type:             ExpectTypePtr,
					Typ:              getTyp(reflect.TypeOf((*int)(nil))),
					Size:             reflect.TypeOf((*int)(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{ // *int
					Type:             ExpectTypeInt,
					Typ:              getTyp(reflect.TypeOf(int(0))),
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: Ptr(Ptr(int(0))),
			ExpectStack: []stackFrame[string]{
				{ // *
					Type:             ExpectTypePtr,
					Typ:              getTyp(reflect.TypeOf((**int)(nil))),
					Size:             reflect.TypeOf((**int)(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{ // **
					Type:             ExpectTypePtr,
					Typ:              getTyp(reflect.TypeOf((*int)(nil))),
					Size:             reflect.TypeOf(((*int)(nil))).Size(),
					ParentFrameIndex: 0,
				},
				{ // **int
					Type:             ExpectTypeInt,
					Typ:              getTyp(reflect.TypeOf(int(0))),
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 1,
				},
			},
		},
		{
			Input: testImplJSONUnmarshaler{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeJSONUnmarshaler,
					Typ:              getTyp(reflect.TypeOf(testImplJSONUnmarshaler{})),
					RType:            reflect.TypeOf(testImplJSONUnmarshaler{}),
					Size:             reflect.TypeOf(testImplJSONUnmarshaler{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: &testImplJSONUnmarshaler{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeJSONUnmarshaler,
					Typ:              getTyp(reflect.TypeOf(&testImplTextUnmarshaler{})),
					RType:            reflect.TypeOf(&testImplJSONUnmarshaler{}),
					Size:             reflect.TypeOf(&testImplJSONUnmarshaler{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: testImplTextUnmarshaler{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeTextUnmarshaler,
					Typ:              getTyp(reflect.TypeOf(testImplTextUnmarshaler{})),
					RType:            reflect.TypeOf(testImplTextUnmarshaler{}),
					Size:             reflect.TypeOf(testImplTextUnmarshaler{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: &testImplTextUnmarshaler{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeTextUnmarshaler,
					Typ:              getTyp(reflect.TypeOf(testImplTextUnmarshaler{})),
					RType:            reflect.TypeOf(&testImplTextUnmarshaler{}),
					Size:             reflect.TypeOf(&testImplTextUnmarshaler{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: []testImplJSONUnmarshaler{},
			ExpectStack: []stackFrame[string]{
				{
					Type: ExpectTypeSlice,
					Typ: getTyp(reflect.TypeOf(
						[]testImplJSONUnmarshaler(nil),
					)),
					Size: reflect.TypeOf(
						[]testImplJSONUnmarshaler(nil),
					).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeJSONUnmarshaler,
					Typ:              getTyp(reflect.TypeOf(testImplJSONUnmarshaler{})),
					RType:            reflect.TypeOf(testImplJSONUnmarshaler{}),
					Size:             reflect.TypeOf(testImplJSONUnmarshaler{}).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: []testImplTextUnmarshaler{},
			ExpectStack: []stackFrame[string]{
				{
					Type: ExpectTypeSlice,
					Typ: getTyp(reflect.TypeOf(
						[]testImplTextUnmarshaler(nil)),
					),
					Size: reflect.TypeOf(
						[]testImplTextUnmarshaler(nil),
					).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeTextUnmarshaler,
					Typ:              getTyp(reflect.TypeOf(testImplTextUnmarshaler{})),
					RType:            reflect.TypeOf(testImplTextUnmarshaler{}),
					Size:             reflect.TypeOf(testImplTextUnmarshaler{}).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: testImplUnmarshalerWithUnmarshalerFields{},
			ExpectStack: []stackFrame[string]{
				{
					Type: ExpectTypeJSONUnmarshaler,
					Typ: getTyp(reflect.TypeOf(
						testImplUnmarshalerWithUnmarshalerFields{},
					)),
					RType: reflect.TypeOf(testImplUnmarshalerWithUnmarshalerFields{}),
					Size: reflect.TypeOf(
						testImplUnmarshalerWithUnmarshalerFields{},
					).Size(),
					ParentFrameIndex: noParentFrame,
				},
			},
		},
		{
			Input: S4{},
			ExpectStack: []stackFrame[string]{
				{ // S4
					Fields: []fieldStackFrame{
						{Name: "name", FrameIndex: 1},
						{Name: "unmar", FrameIndex: 2},
						{Name: "tail", FrameIndex: 3},
					},
					Type:             ExpectTypeStruct,
					Typ:              getTyp(reflect.TypeOf(S4{})),
					RType:            reflect.TypeOf(S4{}),
					Size:             reflect.TypeOf(S4{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{ // S4.Name
					Type:             ExpectTypeStr,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 0,
					Offset:           tpS4.Field(0).Offset,
				},
				{ // S4.Unmarshaler
					Type:             ExpectTypeJSONUnmarshaler,
					Typ:              getTyp(reflect.TypeOf(testImplJSONUnmarshaler{})),
					Size:             reflect.TypeOf(testImplJSONUnmarshaler{}).Size(),
					RType:            reflect.TypeOf(testImplJSONUnmarshaler{}),
					ParentFrameIndex: 0,
					Offset:           tpS4.Field(1).Offset,
				},
				{ // S4.Tail
					Type:             ExpectTypeSliceInt,
					Typ:              getTyp(reflect.TypeOf([]int(nil))),
					Size:             reflect.TypeOf([]int(nil)).Size(),
					ParentFrameIndex: 0,
					Offset:           tpS4.Field(2).Offset,
				},
			},
		},
		{
			Input: S5{},
			ExpectStack: []stackFrame[string]{
				{ // S5
					Fields: []fieldStackFrame{
						{Name: "name", FrameIndex: 1},
						{Name: "unmar", FrameIndex: 2},
						{Name: "tail", FrameIndex: 3},
					},
					Type:             ExpectTypeStruct,
					Typ:              getTyp(reflect.TypeOf(S5{})),
					RType:            reflect.TypeOf(S5{}),
					Size:             reflect.TypeOf(S5{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{ // S5.Name
					Type:             ExpectTypeStr,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 0,
					Offset:           tpS5.Field(0).Offset,
				},
				{ // S5.Unmarshaler
					Type:             ExpectTypeJSONUnmarshaler,
					Typ:              getTyp(reflect.TypeOf(&testImplJSONUnmarshaler{})),
					Size:             reflect.TypeOf(&testImplJSONUnmarshaler{}).Size(),
					RType:            reflect.TypeOf(&testImplJSONUnmarshaler{}),
					ParentFrameIndex: 0,
					Offset:           tpS5.Field(1).Offset,
				},
				{ // S5.Tail
					Type:             ExpectTypeSliceInt,
					Typ:              getTyp(reflect.TypeOf([]int(nil))),
					Size:             reflect.TypeOf([]int(nil)).Size(),
					ParentFrameIndex: 0,
					Offset:           tpS5.Field(2).Offset,
				},
			},
		},
		{
			Input: SStringString{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(SStringString{})),
					RType: reflect.TypeOf(SStringString{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "String"},
					},
					Size:             reflect.TypeOf(SStringString{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeStrString,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: SStringString{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(SStringString{})),
					RType: reflect.TypeOf(SStringString{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "String"},
					},
					Size:             reflect.TypeOf(SStringString{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeStrString,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: SStringBool{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(SStringBool{})),
					RType: reflect.TypeOf(SStringBool{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "Bool"},
					},
					Size:             reflect.TypeOf(SStringBool{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeBoolString,
					Typ:              getTyp(reflect.TypeOf(bool(false))),
					Size:             reflect.TypeOf(bool(false)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: SStringFloat32{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(SStringFloat32{})),
					RType: reflect.TypeOf(SStringFloat32{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "Float32"},
					},
					Size:             reflect.TypeOf(SStringFloat32{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeFloat32String,
					Typ:              getTyp(reflect.TypeOf(float32(0))),
					Size:             reflect.TypeOf(float32(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: SStringFloat64{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(SStringFloat64{})),
					RType: reflect.TypeOf(SStringFloat64{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "Float64"},
					},
					Size:             reflect.TypeOf(SStringFloat64{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeFloat64String,
					Typ:              getTyp(reflect.TypeOf(float64(0))),
					Size:             reflect.TypeOf(float64(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: SStringInt{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(SStringInt{})),
					RType: reflect.TypeOf(SStringInt{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "Int"},
					},
					Size:             reflect.TypeOf(SStringInt{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeIntString,
					Typ:              getTyp(reflect.TypeOf(int(0))),
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: SStringInt8{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(SStringInt8{})),
					RType: reflect.TypeOf(SStringInt8{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "Int8"},
					},
					Size:             reflect.TypeOf(SStringInt8{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeInt8String,
					Typ:              getTyp(reflect.TypeOf(int8(0))),
					Size:             reflect.TypeOf(int8(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: SStringInt16{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(SStringInt16{})),
					RType: reflect.TypeOf(SStringInt16{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "Int16"},
					},
					Size:             reflect.TypeOf(SStringInt16{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeInt16String,
					Typ:              getTyp(reflect.TypeOf(int16(0))),
					Size:             reflect.TypeOf(int16(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: SStringInt32{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(SStringInt32{})),
					RType: reflect.TypeOf(SStringInt32{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "Int32"},
					},
					Size:             reflect.TypeOf(SStringInt32{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeInt32String,
					Typ:              getTyp(reflect.TypeOf(int32(0))),
					Size:             reflect.TypeOf(int32(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: SStringInt64{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(SStringInt64{})),
					RType: reflect.TypeOf(SStringInt64{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "Int64"},
					},
					Size:             reflect.TypeOf(SStringInt64{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeInt64String,
					Typ:              getTyp(reflect.TypeOf(int64(0))),
					Size:             reflect.TypeOf(int64(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: SStringUint{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(SStringUint{})),
					RType: reflect.TypeOf(SStringUint{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "Uint"},
					},
					Size:             reflect.TypeOf(SStringUint{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeUintString,
					Typ:              getTyp(reflect.TypeOf(uint(0))),
					Size:             reflect.TypeOf(uint(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: SStringUint8{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(SStringUint8{})),
					RType: reflect.TypeOf(SStringUint8{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "Uint8"},
					},
					Size:             reflect.TypeOf(SStringUint8{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeUint8String,
					Typ:              getTyp(reflect.TypeOf(uint8(0))),
					Size:             reflect.TypeOf(uint8(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: SStringUint16{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(SStringUint16{})),
					RType: reflect.TypeOf(SStringUint16{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "Uint16"},
					},
					Size:             reflect.TypeOf(SStringUint16{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeUint16String,
					Typ:              getTyp(reflect.TypeOf(uint16(0))),
					Size:             reflect.TypeOf(uint16(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: SStringUint32{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(SStringUint32{})),
					RType: reflect.TypeOf(SStringUint32{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "Uint32"},
					},
					Size:             reflect.TypeOf(SStringUint32{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeUint32String,
					Typ:              getTyp(reflect.TypeOf(uint32(0))),
					Size:             reflect.TypeOf(uint32(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: SStringUint64{},
			ExpectStack: []stackFrame[string]{
				{
					Type:  ExpectTypeStruct,
					Typ:   getTyp(reflect.TypeOf(SStringUint64{})),
					RType: reflect.TypeOf(SStringUint64{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "Uint64"},
					},
					Size:             reflect.TypeOf(SStringUint64{}).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:             ExpectTypeUint64String,
					Typ:              getTyp(reflect.TypeOf(uint64(0))),
					Size:             reflect.TypeOf(uint64(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: []SRecurSlice{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSlice,
					Typ:              getTyp(reflect.TypeOf([]SRecurSlice(nil))),
					Size:             reflect.TypeOf([]SRecurSlice(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:  ExpectTypeStructRecur,
					Typ:   getTyp(reflect.TypeOf(SRecurSlice{})),
					RType: reflect.TypeOf(SRecurSlice{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 2, Name: "ID"},
						{FrameIndex: 3, Name: "Recursion"},
					},
					RecursionStack:   make([]recursionStackFrame, 0, 64),
					Size:             reflect.TypeOf(SRecurSlice{}).Size(),
					ParentFrameIndex: 0,
				},
				{
					Type:             ExpectTypeStr,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					Offset:           reflect.TypeOf(SRecurSlice{}).Field(0).Offset,
					ParentFrameIndex: 1,
				},
				{
					Type:             ExpectTypeSliceRecur,
					Typ:              getTyp(reflect.TypeOf([]SRecurSlice(nil))),
					Size:             reflect.TypeOf([]SRecurSlice(nil)).Size(),
					Offset:           reflect.TypeOf(SRecurSlice{}).Field(1).Offset,
					RecurFrame:       1,
					ParentFrameIndex: 1,
				},
			},
		},
		{
			Input: []SRecurMap{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSlice,
					Typ:              getTyp(reflect.TypeOf([]SRecurMap(nil))),
					Size:             reflect.TypeOf([]SRecurMap(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:  ExpectTypeStructRecur,
					Typ:   getTyp(reflect.TypeOf(SRecurMap{})),
					RType: reflect.TypeOf(SRecurMap{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 2, Name: "ID"},
						{FrameIndex: 3, Name: "Recursion"},
					},
					RecursionStack:   make([]recursionStackFrame, 0, 64),
					Size:             reflect.TypeOf(SRecurMap{}).Size(),
					ParentFrameIndex: 0,
				},
				{
					Type:             ExpectTypeStr,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					Offset:           reflect.TypeOf(SRecurMap{}).Field(0).Offset,
					ParentFrameIndex: 1,
				},
				{
					Type: ExpectTypeMapRecur,
					Typ: getTyp(reflect.TypeOf(
						map[string]SRecurMap(nil),
					)),
					MapValueType: getTyp(reflect.TypeOf(
						SRecurMap{},
					)),
					MapCanUseAssignFaststr: true,
					Size: reflect.TypeOf(
						map[string]SRecurMap(nil),
					).Size(),
					Offset:           reflect.TypeOf(SRecurMap{}).Field(1).Offset,
					RecurFrame:       1,
					ParentFrameIndex: 1,
				},
				{
					Type:             ExpectTypeStr,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 3,
				},
			},
		},
		{
			Input: []SRecurPtr{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSlice,
					Typ:              getTyp(reflect.TypeOf([]SRecurPtr(nil))),
					Size:             reflect.TypeOf([]SRecurPtr(nil)).Size(),
					ParentFrameIndex: noParentFrame,
				},
				{
					Type:  ExpectTypeStructRecur,
					Typ:   getTyp(reflect.TypeOf(SRecurPtr{})),
					RType: reflect.TypeOf(SRecurPtr{}),
					Fields: []fieldStackFrame{
						{FrameIndex: 2, Name: "ID"},
						{FrameIndex: 3, Name: "Recursion"},
					},
					RecursionStack:   make([]recursionStackFrame, 0, 64),
					Size:             reflect.TypeOf(SRecurPtr{}).Size(),
					ParentFrameIndex: 0,
				},
				{
					Type:             ExpectTypeStr,
					Typ:              getTyp(reflect.TypeOf(string(""))),
					Size:             reflect.TypeOf(string("")).Size(),
					Offset:           reflect.TypeOf(SRecurPtr{}).Field(0).Offset,
					ParentFrameIndex: 1,
				},
				{
					Type:             ExpectTypePtrRecur,
					Typ:              getTyp(reflect.TypeOf((*SRecurPtr)(nil))),
					Size:             reflect.TypeOf((*SRecurPtr)(nil)).Size(),
					Offset:           reflect.TypeOf(SRecurPtr{}).Field(1).Offset,
					RecurFrame:       1,
					ParentFrameIndex: 1,
				},
			},
		},
	} {
		t.Run(fmt.Sprintf("%T", td.Input), func(t *testing.T) {
			actual, err := appendTypeToStack[string](
				nil, reflect.TypeOf(td.Input), DefaultInitOptions,
			)
			require.NoError(t, err)
			require.Equal(t, len(td.ExpectStack), len(actual),
				"unexpected number of frames:", actual)
			for i, expect := range td.ExpectStack {
				require.Equal(t, expect, actual[i], "at index %d", i)
			}
		})
	}
}

// testImplJSONUnmarshaler implements encoding/json.Unmarshaler for testing purposes.
type testImplJSONUnmarshaler struct{ LenBytes int }

func (t *testImplJSONUnmarshaler) UnmarshalJSON(data []byte) error {
	t.LenBytes = len(data)
	return nil
}

var _ json.Unmarshaler = &testImplJSONUnmarshaler{}

// testImplTextUnmarshaler implements encoding.TextUnmarshaler for testing purposes.
type testImplTextUnmarshaler struct{ LenBytes int }

func (t *testImplTextUnmarshaler) UnmarshalText(text []byte) error {
	t.LenBytes = len(text)
	return nil
}

var _ encoding.TextUnmarshaler = &testImplTextUnmarshaler{}

// testImplUnmarshalerWithUnmarshalerFields implements encoding/json.Unmarshaler
// and encoding.TextUnmarshaler but also features a field that implements said
// interfaces for testing purposes.
type testImplUnmarshalerWithUnmarshalerFields struct {
	InnerJSON json.Unmarshaler
	InnerText encoding.TextUnmarshaler
}

func (t *testImplUnmarshalerWithUnmarshalerFields) UnmarshalJSON(data []byte) error {
	return t.InnerJSON.UnmarshalJSON(data)
}

func (t *testImplUnmarshalerWithUnmarshalerFields) UnmarshalText(text []byte) error {
	return t.InnerText.UnmarshalText(text)
}

var (
	_ json.Unmarshaler         = &testImplUnmarshalerWithUnmarshalerFields{}
	_ encoding.TextUnmarshaler = &testImplUnmarshalerWithUnmarshalerFields{}
)

func Ptr[T any](v T) *T { return &v }

func TestDetermineJSONUnmarshalerSupport(t *testing.T) {
	type testStruct struct {
		Name   string   `json:"name"`
		Number int      `json:"number"`
		Tags   []string `json:"tags"`
	}

	for _, td := range []struct {
		Input  reflect.Type
		Expect interfaceSupport
	}{
		{
			Input:  reflect.TypeOf(int(0)),
			Expect: interfaceSupportNone,
		},
		{
			Input:  reflect.TypeOf(Ptr(int(0))),
			Expect: interfaceSupportNone,
		},
		{
			Input:  reflect.TypeOf(testImplJSONUnmarshaler{}),
			Expect: interfaceSupportPtr,
		},
		{
			Input:  reflect.TypeOf(Ptr(testImplJSONUnmarshaler{})),
			Expect: interfaceSupportCopy,
		},
		{
			Input:  reflect.TypeOf(Ptr(testStruct{})),
			Expect: interfaceSupportNone,
		},
	} {
		t.Run(td.Input.String(), func(t *testing.T) {
			require.Equal(t, td.Expect, determineJSONUnmarshalerSupport(td.Input))
		})
	}
}

func TestDetermineTextUnmarshalerSupport(t *testing.T) {
	type testStruct struct {
		Name   string   `json:"name"`
		Number int      `json:"number"`
		Tags   []string `json:"tags"`
	}

	for _, td := range []struct {
		Input  reflect.Type
		Expect interfaceSupport
	}{
		{
			Input:  reflect.TypeOf(int(0)),
			Expect: interfaceSupportNone,
		},
		{
			Input:  reflect.TypeOf(Ptr(int(0))),
			Expect: interfaceSupportNone,
		},
		{
			Input:  reflect.TypeOf(testImplTextUnmarshaler{}),
			Expect: interfaceSupportPtr,
		},
		{
			Input:  reflect.TypeOf(Ptr(testImplTextUnmarshaler{})),
			Expect: interfaceSupportCopy,
		},
		{
			Input:  reflect.TypeOf(Ptr(testStruct{})),
			Expect: interfaceSupportNone,
		},
	} {
		t.Run(td.Input.String(), func(t *testing.T) {
			require.Equal(t, td.Expect, determineTextUnmarshalerSupport(td.Input))
		})
	}
}
