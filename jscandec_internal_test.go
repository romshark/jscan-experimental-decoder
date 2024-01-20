package jscandec

import (
	_ "embed"
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

	for _, td := range []struct {
		Input       any
		ExpectStack []stackFrame
	}{
		{
			Input: string(""),
			ExpectStack: []stackFrame{
				{
					Size:             reflect.TypeOf(string("")).Size(),
					Type:             ExpectTypeStr,
					ParentFrameIndex: -1,
				},
			},
		},
		{
			Input: int(0),
			ExpectStack: []stackFrame{
				{
					Size:             reflect.TypeOf(int(0)).Size(),
					Type:             ExpectTypeInt,
					ParentFrameIndex: -1,
				},
			},
		},
		{
			Input: S1{},
			ExpectStack: []stackFrame{
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
					Offset:           8,
				},
			},
		},
		{
			Input: S2{},
			ExpectStack: []stackFrame{
				{ // S1
					Fields: []fieldStackFrame{
						{FrameIndex: 1, Name: "S1"},
						{FrameIndex: 4, Name: "Bar"},
						{FrameIndex: 6, Name: "bazz"},
					},
					Size:             reflect.TypeOf(S2{}).Size(),
					Type:             ExpectTypeStruct,
					ParentFrameIndex: -1,
				},
				{ // S1.S2
					Fields: []fieldStackFrame{
						{FrameIndex: 2, Name: "foo"},
						{FrameIndex: 3, Name: "bar"},
					},
					Size:             reflect.TypeOf(S1{}).Size(),
					Type:             ExpectTypeStruct,
					ParentFrameIndex: 0,
				},
				{ // S1.S2.Foo
					Size:             reflect.TypeOf(int(0)).Size(),
					Type:             ExpectTypeInt,
					ParentFrameIndex: 1,
				},
				{ // S1.S2.Bar
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
			ExpectStack: []stackFrame{
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
			ExpectStack: []stackFrame{
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
			ExpectStack: []stackFrame{
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
	} {
		t.Run(fmt.Sprintf("%T", td.Input), func(t *testing.T) {
			actual := appendTypeToStack(nil, reflect.TypeOf(td.Input))
			require.Equal(t, len(td.ExpectStack), len(actual),
				"unexpected number of frames:", actual)
			for i, expect := range td.ExpectStack {
				require.Equal(t, expect, actual[i], "at index %d", i)
			}
		})
	}
}
