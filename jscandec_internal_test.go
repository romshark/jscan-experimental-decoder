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
					Size:             reflect.TypeOf(string("")).Size(),
					Type:             ExpectTypeStr,
					ParentFrameIndex: -1,
				},
			},
		},
		{
			Input: int(0),
			ExpectStack: []stackFrame[string]{
				{
					Size:             reflect.TypeOf(int(0)).Size(),
					Type:             ExpectTypeInt,
					ParentFrameIndex: -1,
				},
			},
		},
		{
			Input: S1{},
			ExpectStack: []stackFrame[string]{
				{ // S1
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "foo"},
						{FrameIndex: 2, Name: "bar"},
					},
					Size:             reflect.TypeOf(S1{}).Size(),
					Type:             ExpectTypeStruct,
					ParentFrameIndex: -1,
				},
				{ // S1.Foo
					Size:             reflect.TypeOf(int(0)).Size(),
					Type:             ExpectTypeInt,
					ParentFrameIndex: 0,
				},
				{ // S1.Bar
					Size:             reflect.TypeOf(string("")).Size(),
					Type:             ExpectTypeStr,
					ParentFrameIndex: 0,
					Offset:           reflect.TypeOf(S1{}).Field(1).Offset,
				},
			},
		},
		{
			Input: S2{},
			ExpectStack: []stackFrame[string]{
				{ // S2
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "S1"},
						{FrameIndex: 4, Name: "Bar"},
						{FrameIndex: 6, Name: "bazz"},
					},
					Size:             reflect.TypeOf(S2{}).Size(),
					Type:             ExpectTypeStruct,
					ParentFrameIndex: -1,
				},
				{ // S2.S1
					Fields: []fieldStackFrame{
						{FrameIndex: 2, Name: "foo"},
						{FrameIndex: 3, Name: "bar"},
					},
					Size:             reflect.TypeOf(S1{}).Size(),
					Type:             ExpectTypeStruct,
					ParentFrameIndex: 0,
				},
				{ // S2.S1.Foo
					Size:             reflect.TypeOf(int(0)).Size(),
					Type:             ExpectTypeInt,
					ParentFrameIndex: 1,
				},
				{ // S2.S1.Bar
					Size:             reflect.TypeOf(string("")).Size(),
					Type:             ExpectTypeStr,
					ParentFrameIndex: 1,
					Offset:           reflect.TypeOf(S1{}).Field(1).Offset,
				},
				{ // S2.Bar
					Size:             reflect.TypeOf([]string{}).Size(),
					Type:             ExpectTypeSlice,
					ParentFrameIndex: 0,
					Offset:           reflect.TypeOf(S2{}).Field(1).Offset,
				},
				{ // S2.Bar[]
					Size:             reflect.TypeOf(string("")).Size(),
					Type:             ExpectTypeStr,
					ParentFrameIndex: 4,
				},
				{ // S2.Bazz
					Size:             reflect.TypeOf(int(0)).Size(),
					Type:             ExpectTypeInt,
					ParentFrameIndex: 0,
					Offset:           reflect.TypeOf(S2{}).Field(3).Offset,
				},
			},
		},
		{
			Input: []byte{},
			ExpectStack: []stackFrame[string]{
				{
					Size:             reflect.TypeOf([]byte{}).Size(),
					Type:             ExpectTypeSlice,
					ParentFrameIndex: -1,
				},
				{
					Size:             reflect.TypeOf(uint8(0)).Size(),
					Type:             ExpectTypeUint8,
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: [][][]string{},
			ExpectStack: []stackFrame[string]{
				{
					Size:             reflect.TypeOf([][][]string{}).Size(),
					Type:             ExpectTypeSlice,
					ParentFrameIndex: -1,
				},
				{
					Size:             reflect.TypeOf([][]string{}).Size(),
					Type:             ExpectTypeSlice,
					ParentFrameIndex: 0,
				},
				{
					Size:             reflect.TypeOf([]string{}).Size(),
					Type:             ExpectTypeSlice,
					ParentFrameIndex: 1,
				},
				{
					Size:             reflect.TypeOf(string("")).Size(),
					Type:             ExpectTypeStr,
					ParentFrameIndex: 2,
				},
			},
		},
		{
			Input: []S1{},
			ExpectStack: []stackFrame[string]{
				{ // []
					Size:             reflect.TypeOf([]S1{}).Size(),
					Type:             ExpectTypeSlice,
					ParentFrameIndex: -1,
				},
				{ // []S1
					Fields: []fieldStackFrame{
						{FrameIndex: 2, Name: "foo"},
						{FrameIndex: 3, Name: "bar"},
					},
					Size:             reflect.TypeOf(S1{}).Size(),
					Type:             ExpectTypeStruct,
					ParentFrameIndex: 0,
				},
				{ // []S1.Foo
					Size:             reflect.TypeOf(int(0)).Size(),
					Type:             ExpectTypeInt,
					ParentFrameIndex: 1,
				},
				{ // []S1.Bar
					Size:             reflect.TypeOf(string("")).Size(),
					Type:             ExpectTypeStr,
					ParentFrameIndex: 1,
					Offset:           8,
				},
			},
		},
		{
			Input: [4]int{},
			ExpectStack: []stackFrame[string]{
				{
					Size:             reflect.TypeOf([4]int{}).Size(),
					Type:             ExpectTypeArray,
					ParentFrameIndex: -1,
				},
				{
					Size:             reflect.TypeOf(int(0)).Size(),
					Type:             ExpectTypeInt,
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[string]string{},
			ExpectStack: []stackFrame[string]{
				{
					RType:            reflect.TypeOf(map[string]string{}),
					Size:             reflect.TypeOf(map[string]string{}).Size(),
					Type:             ExpectTypeMap,
					ParentFrameIndex: -1,
				},
				{ // Key frame
					Size:             reflect.TypeOf(string("")).Size(),
					Type:             ExpectTypeStr,
					ParentFrameIndex: 0,
				},
				{ // Value frame
					Size:             reflect.TypeOf(string("")).Size(),
					Type:             ExpectTypeStr,
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[testImplTextUnmarshaler]int{},
			ExpectStack: []stackFrame[string]{
				{
					RType:            reflect.TypeOf(map[testImplTextUnmarshaler]int{}),
					Type:             ExpectTypeMap,
					Size:             reflect.TypeOf(map[testImplTextUnmarshaler]int{}).Size(),
					ParentFrameIndex: -1,
				},
				{ // Key
					RType:            reflect.TypeOf(testImplTextUnmarshaler{}),
					Type:             ExpectTypeTextUnmarshaler,
					Size:             reflect.TypeOf(testImplTextUnmarshaler{}).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeInt,
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[int]S1{},
			ExpectStack: []stackFrame[string]{
				{
					RType:            reflect.TypeOf(map[int]S1{}),
					Size:             reflect.TypeOf(map[int]S1{}).Size(),
					Type:             ExpectTypeMap,
					ParentFrameIndex: -1,
				},
				{ // Key frame
					Size:             reflect.TypeOf(int(0)).Size(),
					Type:             ExpectTypeInt,
					ParentFrameIndex: 0,
				},
				{ // S1
					Fields: []fieldStackFrame{
						{FrameIndex: 3, Name: "foo"},
						{FrameIndex: 4, Name: "bar"},
					},
					Size:             reflect.TypeOf(S1{}).Size(),
					Type:             ExpectTypeStruct,
					ParentFrameIndex: 0,
				},
				{ // S1.Foo
					Size:             reflect.TypeOf(int(0)).Size(),
					Type:             ExpectTypeInt,
					ParentFrameIndex: 2,
				},
				{ // S1.Bar
					Size:             reflect.TypeOf(string("")).Size(),
					Type:             ExpectTypeStr,
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
					RType: reflect.TypeOf(map[int]map[string]float32{}),
					Size: reflect.TypeOf(
						map[int]map[string]float32{},
					).Size(),
					ParentFrameIndex: -1,
				},
				{
					Type:             ExpectTypeInt,
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 0,
				},
				{
					Type:             ExpectTypeMap,
					RType:            reflect.TypeOf(map[string]float32{}),
					Size:             reflect.TypeOf(map[string]float32{}).Size(),
					ParentFrameIndex: 0,
				},
				{
					Type:             ExpectTypeStr,
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 2,
				},
				{
					Type:             ExpectTypeFloat32,
					Size:             reflect.TypeOf(float32(0)).Size(),
					ParentFrameIndex: 2,
				},
			},
		},
		{
			Input: map[int8]int8{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeMap,
					RType:            reflect.TypeOf(map[int8]int8{}),
					Size:             reflect.TypeOf(map[int8]int8{}).Size(),
					ParentFrameIndex: -1,
				},
				{ // Key
					Type:             ExpectTypeInt8,
					Size:             reflect.TypeOf(int8(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeInt8,
					Size:             reflect.TypeOf(int8(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[int16]int16{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeMap,
					RType:            reflect.TypeOf(map[int16]int16{}),
					Size:             reflect.TypeOf(map[int16]int16{}).Size(),
					ParentFrameIndex: -1,
				},
				{ // Key
					Type:             ExpectTypeInt16,
					Size:             reflect.TypeOf(int16(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeInt16,
					Size:             reflect.TypeOf(int16(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[int32]int32{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeMap,
					RType:            reflect.TypeOf(map[int32]int32{}),
					Size:             reflect.TypeOf(map[int32]int32{}).Size(),
					ParentFrameIndex: -1,
				},
				{ // Key
					Type:             ExpectTypeInt32,
					Size:             reflect.TypeOf(int32(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeInt32,
					Size:             reflect.TypeOf(int32(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[int64]int64{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeMap,
					RType:            reflect.TypeOf(map[int64]int64{}),
					Size:             reflect.TypeOf(map[int64]int64{}).Size(),
					ParentFrameIndex: -1,
				},
				{ // Key
					Type:             ExpectTypeInt64,
					Size:             reflect.TypeOf(int64(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeInt64,
					Size:             reflect.TypeOf(int64(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[uint]uint{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeMap,
					RType:            reflect.TypeOf(map[uint]uint{}),
					Size:             reflect.TypeOf(map[uint]uint{}).Size(),
					ParentFrameIndex: -1,
				},
				{ // Key
					Type:             ExpectTypeUint,
					Size:             reflect.TypeOf(uint(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeUint,
					Size:             reflect.TypeOf(uint(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[uint8]uint8{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeMap,
					RType:            reflect.TypeOf(map[uint8]uint8{}),
					Size:             reflect.TypeOf(map[uint8]uint8{}).Size(),
					ParentFrameIndex: -1,
				},
				{ // Key
					Type:             ExpectTypeUint8,
					Size:             reflect.TypeOf(uint8(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeUint8,
					Size:             reflect.TypeOf(uint8(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[uint16]uint16{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeMap,
					RType:            reflect.TypeOf(map[uint16]uint16{}),
					Size:             reflect.TypeOf(map[uint16]uint16{}).Size(),
					ParentFrameIndex: -1,
				},
				{ // Key
					Type:             ExpectTypeUint16,
					Size:             reflect.TypeOf(uint16(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeUint16,
					Size:             reflect.TypeOf(uint16(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[uint32]uint32{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeMap,
					RType:            reflect.TypeOf(map[uint32]uint32{}),
					Size:             reflect.TypeOf(map[uint32]uint32{}).Size(),
					ParentFrameIndex: -1,
				},
				{ // Key
					Type:             ExpectTypeUint32,
					Size:             reflect.TypeOf(uint32(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeUint32,
					Size:             reflect.TypeOf(uint32(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: map[uint64]uint64{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeMap,
					RType:            reflect.TypeOf(map[uint64]uint64{}),
					Size:             reflect.TypeOf(map[uint64]uint64{}).Size(),
					ParentFrameIndex: -1,
				},
				{ // Key
					Type:             ExpectTypeUint64,
					Size:             reflect.TypeOf(uint64(0)).Size(),
					ParentFrameIndex: 0,
				},
				{ // Value
					Type:             ExpectTypeUint64,
					Size:             reflect.TypeOf(uint64(0)).Size(),
					ParentFrameIndex: 0,
				},
			},
		},
		{
			Input: S3{},
			ExpectStack: []stackFrame[string]{
				{ // S3
					Fields: []fieldStackFrame{
						{Name: "Any", FrameIndex: 1},
						{Name: "Map", FrameIndex: 2},
						{Name: "Slice", FrameIndex: 5},
					},
					Type:             ExpectTypeStruct,
					Size:             reflect.TypeOf(S3{}).Size(),
					ParentFrameIndex: -1,
				},
				{ // S3.Any
					Type:             ExpectTypeAny,
					Size:             tpEmptyIface.Size(),
					ParentFrameIndex: 0,
					Offset:           tpS3.Field(0).Offset,
				},
				{ // S3.Map
					Type:             ExpectTypeMap,
					Size:             reflect.TypeOf(map[string]any{}).Size(),
					RType:            reflect.TypeOf(map[string]any{}),
					ParentFrameIndex: 0,
					Offset:           tpS3.Field(1).Offset,
				},
				{ // Key frame
					Type:             ExpectTypeStr,
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 2,
				},
				{ // Value frame
					Type:             ExpectTypeAny,
					Size:             tpEmptyIface.Size(),
					ParentFrameIndex: 2,
				},
				{ // S3.Slice
					Size:             reflect.TypeOf([]any{}).Size(),
					Type:             ExpectTypeSlice,
					ParentFrameIndex: 0,
					Offset:           tpS3.Field(2).Offset,
				},
				{ // S3.Slice
					Size:             tpEmptyIface.Size(),
					Type:             ExpectTypeAny,
					ParentFrameIndex: 5,
				},
			},
		},
		{
			Input: Ptr(int(0)),
			ExpectStack: []stackFrame[string]{
				{ // *
					Type:             ExpectTypePtr,
					Size:             reflect.TypeOf(Ptr(int(0))).Size(),
					ParentFrameIndex: -1,
				},
				{ // *int
					Type:             ExpectTypeInt,
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
					Size:             reflect.TypeOf(Ptr(Ptr(int(0)))).Size(),
					ParentFrameIndex: -1,
				},
				{ // **
					Type:             ExpectTypePtr,
					Size:             reflect.TypeOf(Ptr(int(0))).Size(),
					ParentFrameIndex: 0,
				},
				{ // **int
					Type:             ExpectTypeInt,
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
					RType:            reflect.TypeOf(testImplJSONUnmarshaler{}),
					Size:             reflect.TypeOf(testImplJSONUnmarshaler{}).Size(),
					ParentFrameIndex: -1,
				},
			},
		},
		{
			Input: &testImplJSONUnmarshaler{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeJSONUnmarshaler,
					RType:            reflect.TypeOf(&testImplJSONUnmarshaler{}),
					Size:             reflect.TypeOf(&testImplJSONUnmarshaler{}).Size(),
					ParentFrameIndex: -1,
				},
			},
		},
		{
			Input: testImplTextUnmarshaler{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeTextUnmarshaler,
					RType:            reflect.TypeOf(testImplTextUnmarshaler{}),
					Size:             reflect.TypeOf(testImplTextUnmarshaler{}).Size(),
					ParentFrameIndex: -1,
				},
			},
		},
		{
			Input: &testImplTextUnmarshaler{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeTextUnmarshaler,
					RType:            reflect.TypeOf(&testImplTextUnmarshaler{}),
					Size:             reflect.TypeOf(&testImplTextUnmarshaler{}).Size(),
					ParentFrameIndex: -1,
				},
			},
		},
		{
			Input: []testImplJSONUnmarshaler{},
			ExpectStack: []stackFrame[string]{
				{
					Type:             ExpectTypeSlice,
					Size:             reflect.TypeOf([]any{}).Size(),
					ParentFrameIndex: -1,
				},
				{
					Type:             ExpectTypeJSONUnmarshaler,
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
					Type:             ExpectTypeSlice,
					Size:             reflect.TypeOf([]any{}).Size(),
					ParentFrameIndex: -1,
				},
				{
					Type:             ExpectTypeTextUnmarshaler,
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
					Type:  ExpectTypeJSONUnmarshaler,
					RType: reflect.TypeOf(testImplUnmarshalerWithUnmarshalerFields{}),
					Size: reflect.TypeOf(
						testImplUnmarshalerWithUnmarshalerFields{},
					).Size(),
					ParentFrameIndex: -1,
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
					Size:             reflect.TypeOf(S4{}).Size(),
					ParentFrameIndex: -1,
				},
				{ // S4.Name
					Type:             ExpectTypeStr,
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 0,
					Offset:           tpS4.Field(0).Offset,
				},
				{ // S4.Unmarshaler
					Type:             ExpectTypeJSONUnmarshaler,
					Size:             reflect.TypeOf(testImplJSONUnmarshaler{}).Size(),
					RType:            reflect.TypeOf(testImplJSONUnmarshaler{}),
					ParentFrameIndex: 0,
					Offset:           tpS4.Field(1).Offset,
				},
				{ // S4.Tail
					Type:             ExpectTypeSlice,
					Size:             reflect.TypeOf([]int{}).Size(),
					ParentFrameIndex: 0,
					Offset:           tpS4.Field(2).Offset,
				},
				{ // S4.Tail[]
					Type:             ExpectTypeInt,
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 3,
					Offset:           0,
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
					Size:             reflect.TypeOf(S5{}).Size(),
					ParentFrameIndex: -1,
				},
				{ // S5.Name
					Type:             ExpectTypeStr,
					Size:             reflect.TypeOf(string("")).Size(),
					ParentFrameIndex: 0,
					Offset:           tpS5.Field(0).Offset,
				},
				{ // S5.Unmarshaler
					Type:             ExpectTypeJSONUnmarshaler,
					Size:             reflect.TypeOf(&testImplJSONUnmarshaler{}).Size(),
					RType:            reflect.TypeOf(&testImplJSONUnmarshaler{}),
					ParentFrameIndex: 0,
					Offset:           tpS5.Field(1).Offset,
				},
				{ // S5.Tail
					Type:             ExpectTypeSlice,
					Size:             reflect.TypeOf([]int{}).Size(),
					ParentFrameIndex: 0,
					Offset:           tpS5.Field(2).Offset,
				},
				{ // S5.Tail[]
					Type:             ExpectTypeInt,
					Size:             reflect.TypeOf(int(0)).Size(),
					ParentFrameIndex: 3,
					Offset:           0,
				},
			},
		},
	} {
		t.Run(fmt.Sprintf("%T", td.Input), func(t *testing.T) {
			actual := appendTypeToStack[string](nil, reflect.TypeOf(td.Input))
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
