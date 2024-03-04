package jscandec

import (
	"bytes"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unsafe"

	"github.com/romshark/jscan-experimental-decoder/internal/atoi"
	"github.com/romshark/jscan-experimental-decoder/internal/jsonnum"
	"github.com/romshark/jscan-experimental-decoder/internal/unescape"

	"github.com/romshark/jscan/v2"
)

var (
	ErrStringTagOptionOnUnsupportedType = errors.New(
		"invalid use of the `string` tag option on unsupported type",
	)
	ErrNilDest         = errors.New("decoding to nil pointer")
	ErrUnexpectedValue = errors.New("unexpected value")
	ErrUnknownField    = errors.New("unknown field")
	ErrIntegerOverflow = errors.New("integer overflow")
)

type ErrorDecode struct {
	Err      error
	Expected ExpectType
	Index    int
}

func (e ErrorDecode) IsErr() bool { return e.Err != nil }

func (e ErrorDecode) Error() string {
	var s strings.Builder
	s.WriteString("at index ")
	s.WriteString(strconv.Itoa(e.Index))
	s.WriteString(": ")
	s.WriteString(e.Err.Error())
	return s.String()
}

type ExpectType int8

const (
	_ ExpectType = iota

	// ExpectTypeJSONUnmarshaler is any type that implements
	// the encoding/json.Unmarshaler interface
	ExpectTypeJSONUnmarshaler

	// ExpectTypeTextUnmarshaler is any type that implements
	// the encoding.TextUnmarshaler interface
	ExpectTypeTextUnmarshaler

	// ExpectTypePtr is any pointer type
	ExpectTypePtr

	// ExpectTypePtrRecur is any recursive pointer type (used for recursive struct fields)
	ExpectTypePtrRecur

	// ExpectTypeAny is type `any`
	ExpectTypeAny

	// ExpectTypeMap is any map type
	ExpectTypeMap

	// ExpectTypeMapStringString is `map[string]string`
	ExpectTypeMapStringString

	// ExpectTypeMapRecur is any recursive map type (used for recursive struct fields)
	ExpectTypeMapRecur

	// ExpectTypeArray is any array type except zero-length array
	ExpectTypeArray

	// ExpectTypeArrayLen0 is any zero-length array type (like [0]int)
	ExpectTypeArrayLen0

	// ExpectTypeSlice is any slice type
	ExpectTypeSlice

	// ExpectTypeSliceRecur is a recursive slice type (used for recursive struct fields)
	ExpectTypeSliceRecur

	// ExpectTypeSliceEmptyStruct is type `[]struct{}`
	ExpectTypeSliceEmptyStruct

	// ExpectTypeSliceBool is type `[]bool`
	ExpectTypeSliceBool

	// ExpectTypeSliceString is type `[]string`
	ExpectTypeSliceString

	// ExpectTypeSliceInt is type `[]int`
	ExpectTypeSliceInt

	// ExpectTypeSliceInt8 is type `[]int8`
	ExpectTypeSliceInt8

	// ExpectTypeSliceInt16 is type `[]int16`
	ExpectTypeSliceInt16

	// ExpectTypeSliceInt32 is type `[]int32`
	ExpectTypeSliceInt32

	// ExpectTypeSliceInt64 is type `[]int64`
	ExpectTypeSliceInt64

	// ExpectTypeSliceUint is type `[]uint`
	ExpectTypeSliceUint

	// ExpectTypeSliceUint8 is type `[]uint8`
	ExpectTypeSliceUint8

	// ExpectTypeSliceUint16 is type `[]uint16`
	ExpectTypeSliceUint16

	// ExpectTypeSliceUint32 is type `[]uint32`
	ExpectTypeSliceUint32

	// ExpectTypeSliceUint64 is type `[]uint64`
	ExpectTypeSliceUint64

	// ExpectTypeSliceFloat32 is type `[]float32`
	ExpectTypeSliceFloat32

	// ExpectTypeSliceFloat64 is type `[]float64`
	ExpectTypeSliceFloat64

	// ExpectTypeStruct is any struct type except `struct{}`
	ExpectTypeStruct

	// ExpectTypeStruct is any recursive struct type
	ExpectTypeStructRecur

	// ExpectTypeEmptyStruct is type `struct{}`
	ExpectTypeEmptyStruct

	// ExpectTypeBool is type `bool`
	ExpectTypeBool

	// ExpectTypeStr is type `string`
	ExpectTypeStr

	// ExpectTypeFloat32 is type `float32`
	ExpectTypeFloat32

	// ExpectTypeFloat64 is type `float64`
	ExpectTypeFloat64

	// ExpectTypeInt is type `int`
	ExpectTypeInt

	// ExpectTypeInt8 is type `int8`
	ExpectTypeInt8

	// ExpectTypeInt16 is type `int16`
	ExpectTypeInt16

	// ExpectTypeInt32 is type `int32`
	ExpectTypeInt32

	// ExpectTypeInt64 is type `int64`
	ExpectTypeInt64

	// ExpectTypeUint is type `uint`
	ExpectTypeUint

	// ExpectTypeUint8 is type `uint8`
	ExpectTypeUint8

	// ExpectTypeUint16 is type `uint16`
	ExpectTypeUint16

	// ExpectTypeUint32 is type `uint32`
	ExpectTypeUint32

	// ExpectTypeUint64 is type `uint64`
	ExpectTypeUint64

	// ExpectTypeBoolString is type `bool` with `json:",string"` tag
	ExpectTypeBoolString

	// ExpectTypeStrString is type `string` with `json:",string"` tag
	ExpectTypeStrString

	// ExpectTypeFloat32String is type `float32` with `json:",string"` tag
	ExpectTypeFloat32String

	// ExpectTypeFloat64String is type `float32` with `json:",string"` tag
	ExpectTypeFloat64String

	// ExpectTypeIntString is type `int` with `json:",string"` tag
	ExpectTypeIntString

	// ExpectTypeInt8String is type `int8` with `json:",string"` tag
	ExpectTypeInt8String

	// ExpectTypeInt16String is type `int16` with `json:",string"` tag
	ExpectTypeInt16String

	// ExpectTypeInt32String is type `int32` with `json:",string"` tag
	ExpectTypeInt32String

	// ExpectTypeInt64String is type `int64` with `json:",string"` tag
	ExpectTypeInt64String

	// ExpectTypeUintString is type `uint` with `json:",string"` tag
	ExpectTypeUintString

	// ExpectTypeUint8String is type `uint8` with `json:",string"` tag
	ExpectTypeUint8String

	// ExpectTypeUint16String is type `uint16` with `json:",string"` tag
	ExpectTypeUint16String

	// ExpectTypeUint32String is type `uint32` with `json:",string"` tag
	ExpectTypeUint32String

	// ExpectTypeUint64String is type `uint64` with `json:",string"` tag
	ExpectTypeUint64String
)

func (t ExpectType) String() string {
	switch t {
	case ExpectTypeJSONUnmarshaler:
		return "interface{UnmarshalJSON([]byte)error}"
	case ExpectTypeTextUnmarshaler:
		return "interface{UnmarshalText([]byte)error}"
	case ExpectTypePtr:
		return "*"
	case ExpectTypePtrRecur:
		return "*⟲"
	case ExpectTypeAny:
		return "any"
	case ExpectTypeMap:
		return "map"
	case ExpectTypeMapStringString:
		return "map[string]string"
	case ExpectTypeMapRecur:
		return "map⟲"
	case ExpectTypeArray:
		return "array"
	case ExpectTypeArrayLen0:
		return "[0]array"
	case ExpectTypeSlice:
		return "slice"
	case ExpectTypeSliceRecur:
		return "slice⟲"
	case ExpectTypeSliceEmptyStruct:
		return "[]struct{}"
	case ExpectTypeSliceBool:
		return "[]bool"
	case ExpectTypeSliceString:
		return "[]string"
	case ExpectTypeSliceInt:
		return "[]int"
	case ExpectTypeSliceInt8:
		return "[]int8"
	case ExpectTypeSliceInt16:
		return "[]int16"
	case ExpectTypeSliceInt32:
		return "[]int32"
	case ExpectTypeSliceInt64:
		return "[]int64"
	case ExpectTypeSliceUint:
		return "[]uint"
	case ExpectTypeSliceUint8:
		return "[]uint8"
	case ExpectTypeSliceUint16:
		return "[]uint16"
	case ExpectTypeSliceUint32:
		return "[]uint32"
	case ExpectTypeSliceUint64:
		return "[]uint64"
	case ExpectTypeSliceFloat32:
		return "[]float32"
	case ExpectTypeSliceFloat64:
		return "[]float64"
	case ExpectTypeStruct:
		return "struct"
	case ExpectTypeStructRecur:
		return "struct⟲"
	case ExpectTypeEmptyStruct:
		return "struct{}"
	case ExpectTypeBool:
		return "boolean"
	case ExpectTypeStr:
		return "string"
	case ExpectTypeFloat32:
		return "float32"
	case ExpectTypeFloat64:
		return "float64"
	case ExpectTypeInt:
		return "int"
	case ExpectTypeInt8:
		return "int8"
	case ExpectTypeInt16:
		return "int16"
	case ExpectTypeInt32:
		return "int32"
	case ExpectTypeInt64:
		return "int64"
	case ExpectTypeUint:
		return "uint"
	case ExpectTypeUint8:
		return "uint8"
	case ExpectTypeUint16:
		return "uint16"
	case ExpectTypeUint32:
		return "uint32"
	case ExpectTypeUint64:
		return "uint64"
	case ExpectTypeBoolString:
		return "string(boolean)"
	case ExpectTypeStrString:
		return "string(string)"
	case ExpectTypeFloat32String:
		return "string(float32)"
	case ExpectTypeFloat64String:
		return "string(float64)"
	case ExpectTypeIntString:
		return "string(int)"
	case ExpectTypeInt8String:
		return "string(int8)"
	case ExpectTypeInt16String:
		return "string(int16)"
	case ExpectTypeInt32String:
		return "string(int32)"
	case ExpectTypeInt64String:
		return "string(int64)"
	case ExpectTypeUintString:
		return "string(uint)"
	case ExpectTypeUint8String:
		return "string(uint8)"
	case ExpectTypeUint16String:
		return "string(uint16)"
	case ExpectTypeUint32String:
		return "string(uint32)"
	case ExpectTypeUint64String:
		return "string(uint64)"
	}
	return ""
}

// isElemComposite returns false for non-composite slice item types,
// otherwise returns true.
func (t ExpectType) isElemComposite() bool {
	switch t {
	case ExpectTypePtr,
		ExpectTypeArrayLen0, // Zero-length arrays require no memory
		ExpectTypeEmptyStruct,
		ExpectTypeBool,
		ExpectTypeStr,
		ExpectTypeFloat32,
		ExpectTypeFloat64,
		ExpectTypeInt,
		ExpectTypeInt8,
		ExpectTypeInt16,
		ExpectTypeInt32,
		ExpectTypeInt64,
		ExpectTypeUint,
		ExpectTypeUint8,
		ExpectTypeUint16,
		ExpectTypeUint32,
		ExpectTypeUint64:
		return false
		// Types such as `ExpectTypeBoolString` will never be used for slice items.
	}
	return true
}

// fieldStackFrame identifies a field within a struct frame.
type fieldStackFrame struct {
	// FrameIndex defines the stack index of the field's value frame.
	FrameIndex uint32

	// Name defines either the name of the field in the struct
	// or the json struct tag if any.
	Name string
}

type recursionStackFrame struct {
	// Dest stores the destination pointer for the recursive frame to be reset to.
	Dest unsafe.Pointer

	// Offset stores the offset for the recursive frame to be reset to.
	Offset uintptr

	// ContainerFrame stores the index of the recursive container frame.
	ContainerFrame uint32
}

type stackFrame[S []byte | string] struct {
	// Fields is only relevant to structs.
	// For every other type Fields is always nil.
	Fields []fieldStackFrame

	// RType is relevant to JSONUnmarshaler frames only.
	RType reflect.Type

	// MapType and MapValueType are relevant to map frames only.
	MapType, MapValueType *typ

	// Size defines the number of bytes the data would occupy in memory.
	// Size caches reflect.Type.Size() for faster access.
	Size uintptr

	// RecursionStack is only relevant to ExpectTypeStructRecur and
	// keeps track of its recursion through pointers/maps/slices.
	RecursionStack []recursionStackFrame

	// CapOrRecurFrame defines the capacity of the parent array for array item frames.
	// For ExpectTypePtrRecur, ExpectTypeMapRecur and ExpectTypeSliceRecur this
	// is the index of the recursive ExpectTypeStructRecur frame.
	CapOrRecurFrame int

	// Len is relevant to array frames only and defines their current length.
	Len int // Overwritten at runtime

	// Dest defines the destination memory to write the data to.
	// Dest is set at runtime and must be reset on every call to Decode
	// to avoid keeping a pointer to the allocated data and allow the GC
	// to clean it up if necessary.
	Dest unsafe.Pointer // Overwritten at runtime

	// Offset is used at runtime for pointer arithmetics if the decoder is
	// currently inside of an array or slice.
	// For struct fields however, Offset is assigned statically at decoder init time.
	Offset uintptr // Overwritten at runtime

	// ParentFrameIndex defines either the index of the composite parent object
	// in the stack, or noParentFrame.
	ParentFrameIndex uint32

	// Type defines what data type is expected at this frame.
	// Same as Size, Type kind could be taken from reflect.Type but it's
	// slower than storing it here.
	Type ExpectType

	// MapCanUseAssignFaststr is only relevant to map frames and indicates whether
	// mapassign_faststr can be used instead of mapassign.
	MapCanUseAssignFaststr bool
}

// noParentFrame uses math.MaxUint32 because the length of the decoder stack
// is very unlikely to reach 4_294_967_295.
const noParentFrame = math.MaxUint32

// DefaultInitOptions are to be used by default. DO NOT MUTATE.
var DefaultInitOptions = &InitOptions{
	DisallowStringTagOptOnUnsupportedTypes: false,
}

// DefaultOptions are to be used by default. DO NOT MUTATE.
var DefaultOptions = &DecodeOptions{
	DisallowUnknownFields: false,
}

// Unmarshal dynamically allocates new decoder and tokenizer instances and
// unmarshals the JSON contents of s into t.
// Unmarshal is primarily a drop-in replacement for the standard encoding/json.Unmarshal
// and behaves identically except for error messages. To significantly improve
// performance by avoiding dynamic decoder and tokenizer allocation and reflection at
// runtime create a reusable decoder instance using NewDecoder.
func Unmarshal[S []byte | string, T any](s S, t *T) error {
	if t == nil {
		return ErrNilDest
	}
	stack := make([]stackFrame[S], 0, 4)
	var err error
	stack, err = appendTypeToStack(stack, reflect.TypeOf(*t), DefaultInitOptions)
	if err != nil {
		return err
	}
	tokenizer := jscan.NewTokenizer[S](
		len(stack)+1, len(s)/2,
	)
	d := Decoder[S, T]{tokenizer: tokenizer, stackExp: stack}
	d.init()
	if err := d.Decode(s, t, DefaultOptions); err.IsErr() {
		if err.Err == ErrStringTagOptionOnUnsupportedType {
			return nil
		}
		return err.Err
	}
	return nil
}

// Decoder is a reusable decoder instance.
type Decoder[S []byte | string, T any] struct {
	tokenizer    *jscan.Tokenizer[S]
	stackExp     []stackFrame[S]
	parseInt     func(s S) (int, error)
	parseUint    func(s S) (uint, error)
	parseFloat32 func(s S) (float32, error)
	parseFloat64 func(s S) (float64, error)
}

// NewDecoder creates a new reusable decoder instance.
// In case there are multiple decoder instances for different types of T,
// tokenizer is recommended to be shared across them, yet the decoders must
// not be used concurrently!
func NewDecoder[S []byte | string, T any](
	tokenizer *jscan.Tokenizer[S],
	options *InitOptions,
) (*Decoder[S, T], error) {
	d := &Decoder[S, T]{
		tokenizer: tokenizer,
		stackExp:  make([]stackFrame[S], 0, 4),
	}

	var z T
	var err error
	d.stackExp, err = appendTypeToStack(d.stackExp, reflect.TypeOf(z), options)
	if err != nil {
		return nil, err
	}

	d.init()
	return d, nil
}

func (d *Decoder[S, T]) init() {
	// 64-bit system
	d.parseInt = func(s S) (int, error) {
		i, overflow := atoi.I64(s)
		if overflow {
			return 0, ErrIntegerOverflow
		}
		return int(i), nil
	}
	d.parseUint = func(s S) (uint, error) {
		i, overflow := atoi.U64(s)
		if overflow {
			return 0, ErrIntegerOverflow
		}
		return uint(i), nil
	}
	if unsafe.Sizeof(int(0)) != 8 {
		// 32-bit system
		d.parseInt = func(s S) (int, error) {
			i, overflow := atoi.I32(s)
			if overflow {
				return 0, ErrIntegerOverflow
			}
			return int(i), nil
		}
		d.parseUint = func(s S) (uint, error) {
			i, overflow := atoi.U32(s)
			if overflow {
				return 0, ErrIntegerOverflow
			}
			return uint(i), nil
		}
	}

	d.parseFloat32 = func(s S) (float32, error) {
		v, err := strconv.ParseFloat(string(s), 32)
		if err != nil {
			return 0, err
		}
		return float32(v), nil
	}
	d.parseFloat64 = func(s S) (float64, error) {
		v, err := strconv.ParseFloat(string(s), 64)
		if err != nil {
			return 0, err
		}
		return v, nil
	}
	var sz S
	if _, ok := any(sz).([]byte); ok {
		// Avoid copying the slice data
		d.parseFloat32 = func(s S) (float32, error) {
			su := unsafe.String(unsafe.SliceData([]byte(s)), len(s))
			v, err := strconv.ParseFloat(su, 32)
			if err != nil {
				return 0, err
			}
			return float32(v), nil
		}
		d.parseFloat64 = func(s S) (float64, error) {
			su := unsafe.String(unsafe.SliceData([]byte(s)), len(s))
			v, err := strconv.ParseFloat(su, 64)
			if err != nil {
				return 0, err
			}
			return v, nil
		}
	}
}

// appendTypeToStack will recursively flat-append stack frames recursing into t.
func appendTypeToStack[S []byte | string](
	stack []stackFrame[S], t reflect.Type, options *InitOptions,
) ([]stackFrame[S], error) {
	if t == nil {
		return append(stack, stackFrame[S]{
			Type:             ExpectTypeAny,
			Size:             unsafe.Sizeof(struct{ typ, dat uintptr }{}),
			ParentFrameIndex: noParentFrame,
		}), nil
	} else if s := determineJSONUnmarshalerSupport(t); s != interfaceSupportNone {
		return append(stack, stackFrame[S]{
			Type:             ExpectTypeJSONUnmarshaler,
			RType:            t,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		}), nil
	} else if s := determineTextUnmarshalerSupport(t); s != interfaceSupportNone {
		return append(stack, stackFrame[S]{
			Type:             ExpectTypeTextUnmarshaler,
			RType:            t,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		}), nil
	}
	switch t.Kind() {
	case reflect.Interface:
		if t.NumMethod() != 0 {
			// TODO:
			panic("not yet supported")
		}
		return append(stack, stackFrame[S]{
			Type:             ExpectTypeAny,
			Size:             unsafe.Sizeof(struct{ typ, dat uintptr }{}),
			ParentFrameIndex: noParentFrame,
		}), nil
	case reflect.Array:
		if t.Len() == 0 {
			return append(stack, stackFrame[S]{
				Size:             0,
				Type:             ExpectTypeArrayLen0,
				ParentFrameIndex: noParentFrame,
			}), nil
		}

		parentIndex := uint32(len(stack))
		stack = append(stack, stackFrame[S]{
			Size:             t.Size(),
			Type:             ExpectTypeArray,
			ParentFrameIndex: noParentFrame,
		})
		newAtIndex := len(stack)
		var err error
		if stack, err = appendTypeToStack(stack, t.Elem(), options); err != nil {
			return nil, err
		}

		// Link array element to the array frame.
		stack[newAtIndex].ParentFrameIndex = parentIndex
		stack[newAtIndex].CapOrRecurFrame = t.Len()

	case reflect.Slice:
		elem := t.Elem()
		switch elem.Kind() {
		case reflect.Struct:
			if elem.Size() < 1 {
				return append(stack, stackFrame[S]{
					Size:             t.Size(),
					Type:             ExpectTypeSliceEmptyStruct,
					ParentFrameIndex: noParentFrame,
				}), nil
			}
			// Check for recursion
			for i := range stack {
				if stack[i].RType == elem {
					// Recursion of type stack[i] detected.
					// Link recursive frame to the recursion frame.
					stack[i].Type = ExpectTypeStructRecur
					stack[i].RecursionStack = make([]recursionStackFrame, 0, 64)
					return append(stack, stackFrame[S]{
						Type:             ExpectTypeSliceRecur,
						Size:             t.Size(),
						ParentFrameIndex: noParentFrame,
						CapOrRecurFrame:  i,
					}), nil
				}
			}
		case reflect.Bool:
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeSliceBool,
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.String:
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeSliceString,
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Int:
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeSliceInt,
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Int8:
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeSliceInt8,
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Int16:
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeSliceInt16,
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Int32:
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeSliceInt32,
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Int64:
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeSliceInt64,
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Uint:
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeSliceUint,
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Uint8:
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeSliceUint8,
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Uint16:
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeSliceUint16,
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Uint32:
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeSliceUint32,
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Uint64:
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeSliceUint64,
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Float32:
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeSliceFloat32,
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Float64:
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeSliceFloat64,
				ParentFrameIndex: noParentFrame,
			}), nil
		}

		parentIndex := uint32(len(stack))
		stack = append(stack, stackFrame[S]{
			Size:             t.Size(),
			Type:             ExpectTypeSlice,
			ParentFrameIndex: noParentFrame,
		})
		newAtIndex := len(stack)
		var err error
		if stack, err = appendTypeToStack(stack, t.Elem(), options); err != nil {
			return nil, err
		}

		// Link slice element to the slice frame.
		stack[newAtIndex].ParentFrameIndex = parentIndex

	case reflect.Map:
		parentIndex := uint32(len(stack))
		elem := t.Elem()
		if elem.Kind() == reflect.Struct && elem.Size() > 0 {
			// Check for recursion
			for i := range stack {
				if stack[i].RType == elem {
					// Recursion of type stack[i] detected.
					// Link recursive frame to the recursion frame.
					stack[i].Type = ExpectTypeStructRecur
					stack[i].RecursionStack = make([]recursionStackFrame, 0, 64)
					stack = append(stack, stackFrame[S]{
						Type:                   ExpectTypeMapRecur,
						Size:                   t.Size(),
						MapType:                getTyp(t),
						MapValueType:           getTyp(t.Elem()),
						MapCanUseAssignFaststr: canUseAssignFaststr(t),
						ParentFrameIndex:       noParentFrame,
						CapOrRecurFrame:        i,
					})
					{
						newAtIndex := len(stack)
						var err error
						stack, err = appendTypeToStack(stack, t.Key(), options)
						if err != nil {
							return nil, err
						}
						// Link map key to the map frame.
						stack[newAtIndex].ParentFrameIndex = parentIndex
					}
					return stack, nil
				}
			}
		}

		if t.Key().Kind() == reflect.String && t.Elem().Kind() == reflect.String {
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeMapStringString,
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
				RType:            t,
			}), nil
		}

		stack = append(stack, stackFrame[S]{
			Type:                   ExpectTypeMap,
			Size:                   t.Size(),
			MapType:                getTyp(t),
			MapValueType:           getTyp(t.Elem()),
			MapCanUseAssignFaststr: canUseAssignFaststr(t),
			ParentFrameIndex:       noParentFrame,
			RType:                  t,
		})
		{
			newAtIndex := len(stack)
			var err error
			if stack, err = appendTypeToStack(stack, t.Key(), options); err != nil {
				return nil, err
			}
			// Link map key to the map frame.
			stack[newAtIndex].ParentFrameIndex = parentIndex
		}
		{
			newAtIndex := len(stack)
			var err error
			if stack, err = appendTypeToStack(stack, elem, options); err != nil {
				return nil, err
			}
			// Link map value to the map frame.
			stack[newAtIndex].ParentFrameIndex = parentIndex
		}

	case reflect.Struct:
		parentIndex := uint32(len(stack))
		numFields := t.NumField()
		if numFields == 0 {
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeEmptyStruct,
				ParentFrameIndex: noParentFrame,
			}), nil
		}
		stack = append(stack, stackFrame[S]{
			Fields:           make([]fieldStackFrame, 0, numFields),
			Size:             t.Size(),
			Type:             ExpectTypeStruct,
			RType:            t,
			ParentFrameIndex: noParentFrame,
		})

		for i := 0; i < numFields; i++ {
			f := t.Field(i)
			name := f.Name
			optionString := false
			if jsonTag := f.Tag.Get("json"); jsonTag != "" {
				name = jsonTag
				if i := strings.IndexByte(jsonTag, ','); i != -1 {
					name = jsonTag[:i]
					optionName := jsonTag[i+1:]
					optionString = optionName == "string"
				}
				switch name {
				case "":
					// Either there was no tag or no name specified in it.
					name = f.Name
				case "-":
					// Ignore this field.
					continue
				}
			}

			newAtIndex := uint32(len(stack))
			var err error
			stack, err = appendTypeToStack(stack, f.Type, options)
			if err != nil {
				return nil, err
			}
			stack[parentIndex].Fields = append(
				stack[parentIndex].Fields,
				fieldStackFrame{
					Name:       name,
					FrameIndex: newAtIndex,
				},
			)

			// Assign static offset
			stack[newAtIndex].Offset = f.Offset

			// Link the field frame to the parent struct frame.
			stack[newAtIndex].ParentFrameIndex = parentIndex

			if optionString {
				switch stack[newAtIndex].Type {
				case ExpectTypeStr:
					stack[newAtIndex].Type = ExpectTypeStrString
				case ExpectTypeBool:
					stack[newAtIndex].Type = ExpectTypeBoolString
				case ExpectTypeFloat32:
					stack[newAtIndex].Type = ExpectTypeFloat32String
				case ExpectTypeFloat64:
					stack[newAtIndex].Type = ExpectTypeFloat64String
				case ExpectTypeInt:
					stack[newAtIndex].Type = ExpectTypeIntString
				case ExpectTypeInt8:
					stack[newAtIndex].Type = ExpectTypeInt8String
				case ExpectTypeInt16:
					stack[newAtIndex].Type = ExpectTypeInt16String
				case ExpectTypeInt32:
					stack[newAtIndex].Type = ExpectTypeInt32String
				case ExpectTypeInt64:
					stack[newAtIndex].Type = ExpectTypeInt64String
				case ExpectTypeUint:
					stack[newAtIndex].Type = ExpectTypeUintString
				case ExpectTypeUint8:
					stack[newAtIndex].Type = ExpectTypeUint8String
				case ExpectTypeUint16:
					stack[newAtIndex].Type = ExpectTypeUint16String
				case ExpectTypeUint32:
					stack[newAtIndex].Type = ExpectTypeUint32String
				case ExpectTypeUint64:
					stack[newAtIndex].Type = ExpectTypeUint64String
				default:
					// Using tag option `string` on an unsupported type
					if options.DisallowStringTagOptOnUnsupportedTypes {
						return nil, ErrStringTagOptionOnUnsupportedType
					}
				}
			}
		}

	case reflect.Bool:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeBool,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.String:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeStr,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Int:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeInt,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Int8:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeInt8,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Int16:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeInt16,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Int32:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeInt32,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Int64:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeInt64,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Uint:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeUint,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Uint8:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeUint8,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Uint16:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeUint16,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Uint32:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeUint32,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Uint64:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeUint64,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Float32:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeFloat32,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Float64:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeFloat64,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Pointer:
		parentIndex := uint32(len(stack))

		elem := t.Elem()
		if elem.Kind() == reflect.Struct && elem.Size() > 0 {
			// Check for recursion
			for i := range stack {
				if stack[i].RType == elem {
					// Recursion of type stack[i] detected.
					// Link recursive frame to the recursion frame.
					stack[i].Type = ExpectTypeStructRecur
					stack[i].RecursionStack = make([]recursionStackFrame, 0, 64)
					return append(stack, stackFrame[S]{
						Type:             ExpectTypePtrRecur,
						Size:             t.Size(),
						ParentFrameIndex: noParentFrame,
						CapOrRecurFrame:  i,
					}), nil
				}
			}
		}

		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypePtr,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
		newAtIndex := len(stack)
		var err error

		if stack, err = appendTypeToStack(stack, elem, options); err != nil {
			return nil, err
		}
		stack[newAtIndex].ParentFrameIndex = parentIndex

	default:
		return nil, fmt.Errorf("unsupported type: %v", t)
	}
	return stack, nil
}

var (
	tpJSONUnmarshaler = reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
	tpTextUnmarshaler = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
)

type (
	interfaceSupport uint8
)

const (
	interfaceSupportNone interfaceSupport = iota
	interfaceSupportCopy
	interfaceSupportPtr
)

// determineJSONUnmarshalerSupport returns:
//
//   - jsonUnmarshalerSupportNone if t doesn't implement encoding/json.Unmarshaler
//   - jsonUnmarshalerSupportCopy if t implements encoding/json.Unmarshaler
//   - jsonUnmarshalerSupportPtr if pointer to t implements encoding/json.Unmarshaler.
func determineJSONUnmarshalerSupport(t reflect.Type) interfaceSupport {
	if t.AssignableTo(tpJSONUnmarshaler) {
		return interfaceSupportCopy
	}
	if t.Kind() != reflect.Ptr && reflect.PointerTo(t).AssignableTo(tpJSONUnmarshaler) {
		return interfaceSupportPtr
	}
	return interfaceSupportNone
}

// determineTextUnmarshalerSupport returns:
//
//   - textUnmarshalerSupportNone if t doesn't implement encoding/json.Unmarshaler
//   - textUnmarshalerSupportCopy if t implements encoding/json.Unmarshaler
//   - textUnmarshalerSupportPtr if pointer to t implements encoding/json.Unmarshaler.
func determineTextUnmarshalerSupport(t reflect.Type) interfaceSupport {
	if t.AssignableTo(tpTextUnmarshaler) {
		return interfaceSupportCopy
	}
	if t.Kind() != reflect.Ptr && reflect.PointerTo(t).AssignableTo(tpTextUnmarshaler) {
		return interfaceSupportPtr
	}
	return interfaceSupportNone
}

// InitOptions are options for the constructor function NewDecoder[S, T].
type InitOptions struct {
	// DisallowStringTagOptOnUnsupportedTypes will make NewDecoder return
	// ErrStringTagOptionOnUnsupportedType if a `json:",string"` struct tag option is
	// used on an unsupported type.
	DisallowStringTagOptOnUnsupportedTypes bool
}

// DecodeOptions are options for the method *Decoder[S, T].Decode.
type DecodeOptions struct {
	// DisallowUnknownFields will make Decode return ErrUnknownField
	// when encountering an unknown struct field.
	DisallowUnknownFields bool
}

// Decode unmarshals the JSON contents of s into t.
// When S is string the decoder will not copy string values and will instead refer
// to the source string instead since Go strings are guaranteed to be immutable.
// When S is []byte all strings are copied.
//
// Tip: To improve performance reducing dynamic memory allocations define
// options as a variable and pass the pointer. Don't initialize it like this:
//
//	d.Decode(input, &v, &jscandec.DecodeOptions{DisallowUnknownFields: true})
//
// allocate the options to a variable and pass the variable instead:
//
//	d.Decode(input, &v, predefinedOptions)
func (d *Decoder[S, T]) Decode(s S, t *T, options *DecodeOptions) (err ErrorDecode) {
	defer func() {
		for i := range d.stackExp {
			d.stackExp[i].Dest = nil
		}
	}()

	if t == nil {
		return ErrorDecode{Err: ErrNilDest}
	}

	// dumpStack := func(title string) {
	// 	fmt.Printf("STACK « %s » \n", title)
	// 	for i, x := range d.stackExp {
	// 		parent := "-"
	// 		if x.ParentFrameIndex != noParentFrame {
	// 			parent = fmt.Sprintf("%d", x.ParentFrameIndex)
	// 		}
	// 		rType := ""
	// 		if x.RType != nil {
	// 			rType = x.RType.String()
	// 		}
	// 		dest := "0x00000000000"
	// 		if x.Dest != nil {
	// 			dest = fmt.Sprintf("%p", x.Dest)
	// 		}
	// 		fmt.Printf(" %d: %s\tDEST %s\tPAR %s\tOFFSET %d\tSIZE %d\t%s\n",
	// 			i, x.Type, dest, parent, x.Offset, x.Size, rType)
	// 	}
	// 	fmt.Println("------------------------")
	// }
	// dumpStack("init")

	si := uint32(0)
	d.stackExp[0].Dest = unsafe.Pointer(t)
	// fmt.Printf("VROOT %p\n", d.stackExp[0].Dest)

	errTok := d.tokenizer.Tokenize(s, func(tokens []jscan.Token[S]) (exit bool) {
		// ti stands for the token index and points at the current token
		for ti := 0; ti < len(tokens); {
			switch d.stackExp[si].Type {
			case ExpectTypePtr:
				p := unsafe.Pointer(
					uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
				)
				si++
				if d.stackExp[si].Size == 0 {
					*(*unsafe.Pointer)(p) = emptyStructAddr
				} else {
					dp := allocate(d.stackExp[si].Size)
					d.stackExp[si].Dest = dp
					*(*unsafe.Pointer)(p) = dp
				}
				continue
			case ExpectTypePtrRecur:
				panic("TODO")
			case ExpectTypeJSONUnmarshaler:
				goto ON_JSON_UNMARSHALER
			}
			switch tokens[ti].Type {
			case jscan.TokenTypeFalse, jscan.TokenTypeTrue:
				switch d.stackExp[si].Type {
				case ExpectTypeAny:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					v := tokens[ti].Type == jscan.TokenTypeTrue
					**(**interface{})(unsafe.Pointer(&p)) = v
				case ExpectTypeBool:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					*(*bool)(p) = tokens[ti].Type == jscan.TokenTypeTrue
				default:
					err = ErrorDecode{
						Err:   ErrUnexpectedValue,
						Index: tokens[ti].Index,
					}
					return true
				}
				ti++
				goto ON_VAL_END

			case jscan.TokenTypeInteger:
				switch d.stackExp[si].Type {
				case ExpectTypeAny:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					tv := s[tokens[ti].Index:tokens[ti].End]
					var sz S
					var su string
					switch any(sz).(type) {
					case []byte:
						su = unsafe.String(unsafe.SliceData([]byte(tv)), len(tv))
					case string:
						su = string(tv)
					}
					v, errParse := strconv.ParseFloat(su, 64)
					if errParse != nil {
						err = ErrorDecode{
							Err:   errParse,
							Index: tokens[ti].Index,
						}
						return true
					}
					**(**interface{})(unsafe.Pointer(&p)) = v

				case ExpectTypeUint:
					if s[tokens[ti].Index] == '-' {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					if i, e := d.parseUint(s[tokens[ti].Index:tokens[ti].End]); e != nil {
						// Invalid unsigned integer
						err = ErrorDecode{
							Err:   e,
							Index: tokens[ti].Index,
						}
						return true
					} else {
						p := unsafe.Pointer(
							uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
						)
						*(*uint)(p) = i
					}

				case ExpectTypeInt:
					if i, e := d.parseInt(s[tokens[ti].Index:tokens[ti].End]); e != nil {
						// Invalid signed integer
						err = ErrorDecode{
							Err:   e,
							Index: tokens[ti].Index,
						}
						return true
					} else {
						p := unsafe.Pointer(
							uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
						)
						*(*int)(p) = i
					}

				case ExpectTypeUint8:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					if s[tokens[ti].Index] == '-' {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, overflow := atoi.U8(s[tokens[ti].Index:tokens[ti].End])
					if overflow {
						// Invalid 8-bit unsigned integer
						err = ErrorDecode{
							Err:   ErrIntegerOverflow,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*uint8)(p) = v

				case ExpectTypeUint16:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					if s[tokens[ti].Index] == '-' {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, overflow := atoi.U16(s[tokens[ti].Index:tokens[ti].End])
					if overflow {
						// Invalid 16-bit unsigned integer
						err = ErrorDecode{
							Err:   ErrIntegerOverflow,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*uint16)(p) = v

				case ExpectTypeUint32:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					if s[tokens[ti].Index] == '-' {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, overflow := atoi.U32(s[tokens[ti].Index:tokens[ti].End])
					if overflow {
						// Invalid 32-bit unsigned integer
						err = ErrorDecode{
							Err:   ErrIntegerOverflow,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*uint32)(p) = v

				case ExpectTypeUint64:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					if s[tokens[ti].Index] == '-' {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, overflow := atoi.U64(s[tokens[ti].Index:tokens[ti].End])
					if overflow {
						// Invalid 64-bit unsigned integer
						err = ErrorDecode{
							Err:   ErrIntegerOverflow,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*uint64)(p) = v

				case ExpectTypeInt8:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					v, overflow := atoi.I8(s[tokens[ti].Index:tokens[ti].End])
					if overflow {
						// Invalid 8-bit signed integer
						err = ErrorDecode{
							Err:   ErrIntegerOverflow,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*int8)(p) = v

				case ExpectTypeInt16:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					v, overflow := atoi.I16(s[tokens[ti].Index:tokens[ti].End])
					if overflow {
						// Invalid 16-bit signed integer
						err = ErrorDecode{
							Err:   ErrIntegerOverflow,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*int16)(p) = v

				case ExpectTypeInt32:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					v, overflow := atoi.I32(s[tokens[ti].Index:tokens[ti].End])
					if overflow {
						// Invalid 32-bit signed integer
						err = ErrorDecode{
							Err:   ErrIntegerOverflow,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*int32)(p) = v

				case ExpectTypeInt64:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					v, overflow := atoi.I64(s[tokens[ti].Index:tokens[ti].End])
					if overflow {
						// Invalid 64-bit signed integer
						err = ErrorDecode{
							Err:   ErrIntegerOverflow,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*int64)(p) = v

				case ExpectTypeFloat32:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					if tokens[ti].End-tokens[ti].Index < len("16777216") {
						// Numbers below this length are guaranteed to be smaller 1<<24
						// And float32(i32) is faster than calling parseFloat32
						i32, errParse := tokens[ti].Int32(s)
						if errParse != nil {
							err = ErrorDecode{Err: errParse, Index: tokens[ti].Index}
							return true
						}
						*(*float32)(p) = float32(i32)
					} else {
						v, errParse := d.parseFloat32(s[tokens[ti].Index:tokens[ti].End])
						if errParse != nil {
							err = ErrorDecode{Err: errParse, Index: tokens[ti].Index}
							return true
						}
						*(*float32)(p) = v
					}

				case ExpectTypeFloat64:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					if tokens[ti].End-tokens[ti].Index < len("9007199254740992") {
						// Numbers below this length are guaranteed to be smaller 1<<53
						// And float64(i64) is faster than calling parseFloat64
						i64, errParse := tokens[ti].Int64(s)
						if errParse != nil {
							err = ErrorDecode{Err: errParse, Index: tokens[ti].Index}
							return true
						}
						*(*float64)(p) = float64(i64)
					} else {
						v, errParse := d.parseFloat64(s[tokens[ti].Index:tokens[ti].End])
						if errParse != nil {
							err = ErrorDecode{Err: errParse, Index: tokens[ti].Index}
							return true
						}
						*(*float64)(p) = v
					}

				default:
					err = ErrorDecode{
						Err:   ErrUnexpectedValue,
						Index: tokens[ti].Index,
					}
					return true
				}
				ti++
				goto ON_VAL_END

			case jscan.TokenTypeNumber:
				switch d.stackExp[si].Type {
				case ExpectTypeAny:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					tv := s[tokens[ti].Index:tokens[ti].End]
					var sz S
					var su string
					switch any(sz).(type) {
					case []byte:
						su = unsafe.String(unsafe.SliceData([]byte(tv)), len(tv))
					case string:
						su = string(tv)
					}
					v, errParse := strconv.ParseFloat(su, 64)
					if errParse != nil {
						err = ErrorDecode{
							Err:   errParse,
							Index: tokens[ti].Index,
						}
						return true
					}
					**(**interface{})(unsafe.Pointer(&p)) = v

				case ExpectTypeFloat32:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					v, errParse := d.parseFloat32(s[tokens[ti].Index:tokens[ti].End])
					if errParse != nil {
						err = ErrorDecode{Err: errParse, Index: tokens[ti].Index}
						return true
					}
					*(*float32)(p) = v
				case ExpectTypeFloat64:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					v, errParse := d.parseFloat64(s[tokens[ti].Index:tokens[ti].End])
					if errParse != nil {
						err = ErrorDecode{Err: errParse, Index: tokens[ti].Index}
						return true
					}
					*(*float64)(p) = v
				default:
					err = ErrorDecode{Err: ErrUnexpectedValue, Index: tokens[ti].Index}
					return true
				}
				ti++
				goto ON_VAL_END

			case jscan.TokenTypeString:
				p := unsafe.Pointer(
					uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
				)

				switch d.stackExp[si].Type {
				case ExpectTypeTextUnmarshaler:
					u := reflect.NewAt(
						d.stackExp[si].RType, p,
					).Interface().(encoding.TextUnmarshaler)
					tb := unescape.Valid[S, []byte](
						s[tokens[ti].Index+1 : tokens[ti].End-1],
					)
					if errUnmarshal := u.UnmarshalText(tb); errUnmarshal != nil {
						err = ErrorDecode{
							Err:   errUnmarshal,
							Index: tokens[ti].Index,
						}
						return true
					}
				case ExpectTypeAny:
					**(**interface{})(unsafe.Pointer(&p)) = unescape.Valid[S, string](
						s[tokens[ti].Index+1 : tokens[ti].End-1],
					)
				case ExpectTypeStr:
					*(*string)(p) = unescape.Valid[S, string](
						s[tokens[ti].Index+1 : tokens[ti].End-1],
					)
					// fmt.Printf("%d\tSTR %q\tAT %p OFFSET %d\n",
					// 	ti, *(*string)(p), d.stackExp[si].Dest, d.stackExp[si].Offset)
				case ExpectTypeBoolString:
					switch string(s[tokens[ti].Index+1 : tokens[ti].End-1]) {
					case "true":
						*(*bool)(p) = true
					case "false":
						*(*bool)(p) = false
					default:
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
				case ExpectTypeStrString:
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					if len(tv) < len(`\"\"`) ||
						string(tv[0:2]) != `\"` ||
						string(tv[len(tv)-2:]) != `\"` {
						// Too short or not encloused by \"
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					val := tv[2 : len(tv)-2]
					var indexReverseSolidus int
					switch val := any(val).(type) {
					case string:
						indexReverseSolidus = strings.IndexByte(val, '\\')
					case []byte:
						indexReverseSolidus = bytes.IndexByte(val, '\\')
					}
					if indexReverseSolidus != -1 {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*string)(p) = string(val)

				case ExpectTypeFloat32String:
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					_, rc := jsonnum.ReadNumber(tv)
					if rc == jsonnum.ReturnCodeErr {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, errParse := d.parseFloat32(tv)
					if errParse != nil {
						err = ErrorDecode{Err: errParse, Index: tokens[ti].Index}
						return true
					}
					*(*float32)(p) = v
				case ExpectTypeFloat64String:
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					_, rc := jsonnum.ReadNumber(tv)
					if rc == jsonnum.ReturnCodeErr {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, errParse := d.parseFloat64(tv)
					if errParse != nil {
						err = ErrorDecode{Err: errParse, Index: tokens[ti].Index}
						return true
					}
					*(*float64)(p) = v
				case ExpectTypeIntString:
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					_, rc := jsonnum.ReadNumber(tv)
					if rc != jsonnum.ReturnCodeInteger {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, errParse := d.parseInt(tv)
					if errParse != nil {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*int)(p) = v
				case ExpectTypeInt8String:
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					_, rc := jsonnum.ReadNumber(tv)
					if rc != jsonnum.ReturnCodeInteger {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, overflow := atoi.I8(tv)
					if overflow {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*int8)(p) = v
				case ExpectTypeInt16String:
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					_, rc := jsonnum.ReadNumber(tv)
					if rc != jsonnum.ReturnCodeInteger {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, overflow := atoi.I16(tv)
					if overflow {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*int16)(p) = v
				case ExpectTypeInt32String:
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					_, rc := jsonnum.ReadNumber(tv)
					if rc != jsonnum.ReturnCodeInteger {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, overflow := atoi.I32(tv)
					if overflow {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*int32)(p) = v
				case ExpectTypeInt64String:
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					_, rc := jsonnum.ReadNumber(tv)
					if rc != jsonnum.ReturnCodeInteger {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, overflow := atoi.I64(tv)
					if overflow {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*int64)(p) = v
				case ExpectTypeUintString:
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					_, rc := jsonnum.ReadNumber(tv)
					if rc != jsonnum.ReturnCodeInteger || tv[0] == '-' {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, errParse := d.parseUint(tv)
					if errParse != nil {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*uint)(p) = v
				case ExpectTypeUint8String:
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					_, rc := jsonnum.ReadNumber(tv)
					if rc != jsonnum.ReturnCodeInteger || tv[0] == '-' {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, overflow := atoi.U8(tv)
					if overflow {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*uint8)(p) = v
				case ExpectTypeUint16String:
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					_, rc := jsonnum.ReadNumber(tv)
					if rc != jsonnum.ReturnCodeInteger || tv[0] == '-' {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, overflow := atoi.U16(tv)
					if overflow {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*uint16)(p) = v
				case ExpectTypeUint32String:
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					_, rc := jsonnum.ReadNumber(tv)
					if rc != jsonnum.ReturnCodeInteger || tv[0] == '-' {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, overflow := atoi.U32(tv)
					if overflow {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*uint32)(p) = v
				case ExpectTypeUint64String:
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					_, rc := jsonnum.ReadNumber(tv)
					if rc != jsonnum.ReturnCodeInteger || tv[0] == '-' {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					v, overflow := atoi.U64(tv)
					if overflow {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					*(*uint64)(p) = v
				default:
					err = ErrorDecode{
						Err:   ErrUnexpectedValue,
						Index: tokens[ti].Index,
					}
					return true
				}
				// This will either copy from a byte slice or create a substring
				ti++
				goto ON_VAL_END

			case jscan.TokenTypeNull:
				p := unsafe.Pointer(
					uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
				)
				switch d.stackExp[si].Type {
				case ExpectTypeEmptyStruct:
					// Nothing
				case ExpectTypeAny:
					*(*any)(p) = nil
				case ExpectTypeMapStringString:
					*(*map[string]string)(p) = nil
				case ExpectTypeMap:
					*(*map[any]any)(p) = nil
				case ExpectTypeSlice:
					// Skip
				case ExpectTypeStruct, ExpectTypeStructRecur:
					// Skip
				case ExpectTypeBool:
					*(*bool)(p) = zeroBool
				case ExpectTypeStr:
					*(*string)(p) = zeroStr
				case ExpectTypeFloat32:
					*(*float32)(p) = zeroFloat32
				case ExpectTypeFloat64:
					*(*float64)(p) = zeroFloat64
				case ExpectTypeInt:
					*(*int)(p) = zeroInt
				case ExpectTypeInt8:
					*(*int8)(p) = zeroInt8
				case ExpectTypeInt16:
					*(*int16)(p) = zeroInt16
				case ExpectTypeInt32:
					*(*int32)(p) = zeroInt32
				case ExpectTypeInt64:
					*(*int64)(p) = zeroInt64
				case ExpectTypeUint:
					*(*uint)(p) = zeroUint
				case ExpectTypeUint8:
					*(*uint8)(p) = zeroUint8
				case ExpectTypeUint16:
					*(*uint16)(p) = zeroUint16
				case ExpectTypeUint32:
					*(*uint32)(p) = zeroUint32
				case ExpectTypeUint64:
					*(*uint64)(p) = zeroUint64
				case ExpectTypeSliceBool:
					*(*[]bool)(p) = nil
				case ExpectTypeSliceString:
					*(*[]string)(p) = nil
				case ExpectTypeSliceFloat32:
					*(*[]float32)(p) = nil
				case ExpectTypeSliceFloat64:
					*(*[]float64)(p) = nil
				case ExpectTypeSliceInt:
					*(*[]int)(p) = nil
				case ExpectTypeSliceInt8:
					*(*[]int8)(p) = nil
				case ExpectTypeSliceInt16:
					*(*[]int16)(p) = nil
				case ExpectTypeSliceInt32:
					*(*[]int32)(p) = nil
				case ExpectTypeSliceInt64:
					*(*[]int64)(p) = nil
				case ExpectTypeSliceUint:
					*(*[]uint)(p) = nil
				case ExpectTypeSliceUint8:
					*(*[]uint8)(p) = nil
				case ExpectTypeSliceUint16:
					*(*[]uint16)(p) = nil
				case ExpectTypeSliceUint32:
					*(*[]uint32)(p) = nil
				case ExpectTypeSliceUint64:
					*(*[]uint64)(p) = nil
				}
				ti++
				goto ON_VAL_END

			case jscan.TokenTypeArray:
				switch d.stackExp[si].Type {
				case ExpectTypeAny:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					v, tail, errDecode := decodeAny(s, tokens[ti:])
					if errDecode != nil {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					**(**interface{})(unsafe.Pointer(&p)) = v
					ti = len(tokens) - len(tail)
					goto ON_VAL_END

				case ExpectTypeArrayLen0:
					// Ignore this array
					ti = tokens[ti].End + 1
					goto ON_VAL_END

				case ExpectTypeArray:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					if tokens[ti].Elements < 1 {
						ti += 2 // Skip over closing TokenArrayEnd
						// Empty array, no need to allocate memory.
						// Go will automatically allocate it together with its parent
						// and zero it.
						goto ON_VAL_END
					}
					ti++
					si++
					d.stackExp[si].Dest = p
					d.stackExp[si].Len = 0
					d.stackExp[si].Offset = 0

				case ExpectTypeSlice:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					elementSize := d.stackExp[si+1].Size
					elems := uintptr(tokens[ti].Elements)

					if elems == 0 {
						// Allocate empty slice
						emptySlice := sliceHeader{Data: emptyStructAddr, Len: 0, Cap: 0}
						if elementSize > 0 {
							emptySlice.Data = allocate(elementSize)
						}
						*(*sliceHeader)(p) = emptySlice
						ti += 2
						goto ON_VAL_END
					}

					var dp unsafe.Pointer
					if h := *(*sliceHeader)(p); h.Cap < uintptr(tokens[ti].Elements) {
						sh := sliceHeader{Data: emptyStructAddr, Len: elems, Cap: elems}
						allocated := make([]byte, elems*elementSize)
						if h.Len != 0 && d.stackExp[si+1].Type.isElemComposite() {
							// Must copy existing data because it's not guarenteed
							// that the existing data will be fully overwritten.
							copy(allocated, *(*[]byte)(unsafe.Pointer(&sliceHeader{
								Data: h.Data,
								Len:  h.Len * elementSize,
								Cap:  h.Cap * elementSize,
							})))
						}
						sh.Data = unsafe.Pointer(&allocated[0])
						*(*sliceHeader)(p) = sh
						dp = sh.Data
					} else {
						(*sliceHeader)(p).Len = uintptr(tokens[ti].Elements)
						dp = (*sliceHeader)(p).Data
					}
					ti++
					si++
					d.stackExp[si].Dest = dp
					d.stackExp[si].Offset = 0

				case ExpectTypeSliceRecur:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					recursiveFrame := d.stackExp[si].CapOrRecurFrame
					elementSize := d.stackExp[recursiveFrame].Size
					elems := uintptr(tokens[ti].Elements)

					if elems == 0 {
						// Allocate empty slice
						emptySlice := sliceHeader{Data: emptyStructAddr, Len: 0, Cap: 0}
						if elementSize > 0 {
							emptySlice.Data = allocate(elementSize)
						}
						*(*sliceHeader)(p) = emptySlice
						ti += 2
						if len(d.stackExp[recursiveFrame].RecursionStack) < 1 {
							goto ON_VAL_END
						}
						si = uint32(recursiveFrame)
						continue
					}

					var dp unsafe.Pointer
					if h := *(*sliceHeader)(p); h.Cap < uintptr(tokens[ti].Elements) {
						sh := sliceHeader{Data: emptyStructAddr, Len: elems, Cap: elems}
						allocated := make([]byte, elems*elementSize)
						if h.Len != 0 && d.stackExp[si+1].Type.isElemComposite() {
							// Must copy existing data because it's not guarenteed
							// that the existing data will be fully overwritten.
							copy(allocated, *(*[]byte)(unsafe.Pointer(&sliceHeader{
								Data: h.Data,
								Len:  h.Len * elementSize,
								Cap:  h.Cap * elementSize,
							})))
						}
						sh.Data = unsafe.Pointer(&allocated[0])
						*(*sliceHeader)(p) = sh
						dp = sh.Data
						// fmt.Printf("%d\tNEW SLICE\tAT %p %d/%d\n",
						// 	ti, sh.Data, sh.Len, sh.Cap)
					} else {
						(*sliceHeader)(p).Len = uintptr(tokens[ti].Elements)
						dp = (*sliceHeader)(p).Data
					}
					ti++

					// Push recursion stack
					d.stackExp[recursiveFrame].RecursionStack = append(
						d.stackExp[recursiveFrame].RecursionStack, recursionStackFrame{
							Dest:           d.stackExp[recursiveFrame].Dest,
							Offset:         d.stackExp[recursiveFrame].Offset,
							ContainerFrame: si,
						},
					)
					// fmt.Printf("%d\tPUSH SLICE\tON %p OFFSET %d SI %d\n",
					// 	ti,
					// 	d.stackExp[si].Dest,
					// 	d.stackExp[si].Offset,
					// 	si)
					// fmt.Printf("%d\tPUSHED\t\t%v\n",
					// 	ti,
					// 	d.stackExp[recursiveFrame].RecursionStack)

					si = uint32(recursiveFrame)
					d.stackExp[si].Dest = dp
					d.stackExp[si].Offset = 0

				case ExpectTypeSliceEmptyStruct:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					*(*sliceHeader)(p) = sliceHeader{
						Data: emptyStructAddr,
						Len:  uintptr(tokens[ti].Elements),
						Cap:  uintptr(tokens[ti].Elements),
					}
					ti = tokens[ti].End + 1
					goto ON_VAL_END

				case ExpectTypeSliceBool:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					elems := tokens[ti].Elements
					if elems < 1 {
						*(*[]bool)(p) = []bool{}
						ti += 2
						goto ON_VAL_END
					}

					sl := *(*[]bool)(p)
					if cap(sl) < elems {
						sl = make([]bool, elems)
					} else {
						sl = sl[:elems]
					}

					tokens := tokens[ti+1 : tokens[ti].End]
					for i := range tokens {
						switch tokens[i].Type {
						case jscan.TokenTypeNull:
							sl[i] = false
						case jscan.TokenTypeTrue:
							sl[i] = true
						case jscan.TokenTypeFalse:
							sl[i] = false
						default:
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[i].Index,
							}
							return true
						}
					}
					ti += len(tokens) + 2
					*(*[]bool)(p) = sl
					goto ON_VAL_END

				case ExpectTypeSliceString:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					elems := tokens[ti].Elements
					if elems < 1 {
						*(*[]string)(p) = []string{}
						ti += 2
						goto ON_VAL_END
					}

					sl := *(*[]string)(p)
					if cap(sl) < elems {
						sl = make([]string, elems)
					} else {
						sl = sl[:elems]
					}

					tokens := tokens[ti+1 : tokens[ti].End]
					for i := range tokens {
						switch tokens[i].Type {
						case jscan.TokenTypeNull:
							sl[i] = ""
						case jscan.TokenTypeString:
							sl[i] = unescape.Valid[S, string](
								s[tokens[i].Index+1 : tokens[i].End-1],
							)
						default:
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[i].Index,
							}
							return true
						}
					}
					ti += len(tokens) + 2
					*(*[]string)(p) = sl
					goto ON_VAL_END

				case ExpectTypeSliceInt:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					elems := tokens[ti].Elements
					if elems < 1 {
						*(*[]int)(p) = []int{}
						ti += 2
						goto ON_VAL_END
					}

					sl := *(*[]int)(p)
					if cap(sl) < elems {
						sl = make([]int, elems)
					} else {
						sl = sl[:elems]
					}

					tokens := tokens[ti+1 : tokens[ti].End]
					for i := range tokens {
						switch tokens[i].Type {
						case jscan.TokenTypeNull:
							sl[i] = 0
						case jscan.TokenTypeInteger:
							v, errParse := d.parseInt(s[tokens[i].Index:tokens[i].End])
							if errParse != nil {
								err = ErrorDecode{
									Err:   errParse,
									Index: tokens[i].Index,
								}
								return true
							}
							sl[i] = v
						default:
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[i].Index,
							}
							return true
						}
					}
					ti += len(tokens) + 2
					*(*[]int)(p) = sl
					goto ON_VAL_END

				case ExpectTypeSliceInt8:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					elems := tokens[ti].Elements
					if elems < 1 {
						*(*[]int8)(p) = []int8{}
						ti += 2
						goto ON_VAL_END
					}

					sl := *(*[]int8)(p)
					if cap(sl) < elems {
						sl = make([]int8, elems)
					} else {
						sl = sl[:elems]
					}

					tokens := tokens[ti+1 : tokens[ti].End]
					for i := range tokens {
						switch tokens[i].Type {
						case jscan.TokenTypeNull:
							sl[i] = 0
						case jscan.TokenTypeInteger:
							v, overflow := atoi.I8(s[tokens[i].Index:tokens[i].End])
							if overflow {
								err = ErrorDecode{
									Err:   ErrIntegerOverflow,
									Index: tokens[i].Index,
								}
								return true
							}
							sl[i] = v
						default:
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[i].Index,
							}
							return true
						}
					}
					ti += len(tokens) + 2
					*(*[]int8)(p) = sl
					goto ON_VAL_END

				case ExpectTypeSliceInt16:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					elems := tokens[ti].Elements
					if elems < 1 {
						*(*[]int16)(p) = []int16{}
						ti += 2
						goto ON_VAL_END
					}

					sl := *(*[]int16)(p)
					if cap(sl) < elems {
						sl = make([]int16, elems)
					} else {
						sl = sl[:elems]
					}

					tokens := tokens[ti+1 : tokens[ti].End]
					for i := range tokens {
						switch tokens[i].Type {
						case jscan.TokenTypeNull:
							sl[i] = 0
						case jscan.TokenTypeInteger:
							v, overflow := atoi.I16(s[tokens[i].Index:tokens[i].End])
							if overflow {
								err = ErrorDecode{
									Err:   ErrIntegerOverflow,
									Index: tokens[i].Index,
								}
								return true
							}
							sl[i] = v
						default:
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[i].Index,
							}
							return true
						}
					}
					ti += len(tokens) + 2
					*(*[]int16)(p) = sl
					goto ON_VAL_END

				case ExpectTypeSliceInt32:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					elems := tokens[ti].Elements
					if elems < 1 {
						*(*[]int32)(p) = []int32{}
						ti += 2
						goto ON_VAL_END
					}

					sl := *(*[]int32)(p)
					if cap(sl) < elems {
						sl = make([]int32, elems)
					} else {
						sl = sl[:elems]
					}

					tokens := tokens[ti+1 : tokens[ti].End]
					for i := range tokens {
						switch tokens[i].Type {
						case jscan.TokenTypeNull:
							sl[i] = 0
						case jscan.TokenTypeInteger:
							v, overflow := atoi.I32(s[tokens[i].Index:tokens[i].End])
							if overflow {
								err = ErrorDecode{
									Err:   ErrIntegerOverflow,
									Index: tokens[i].Index,
								}
								return true
							}
							sl[i] = v
						default:
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[i].Index,
							}
							return true
						}
					}
					ti += len(tokens) + 2
					*(*[]int32)(p) = sl
					goto ON_VAL_END

				case ExpectTypeSliceInt64:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					elems := tokens[ti].Elements
					if elems < 1 {
						*(*[]int64)(p) = []int64{}
						ti += 2
						goto ON_VAL_END
					}

					sl := *(*[]int64)(p)
					if cap(sl) < elems {
						sl = make([]int64, elems)
					} else {
						sl = sl[:elems]
					}

					tokens := tokens[ti+1 : tokens[ti].End]
					for i := range tokens {
						switch tokens[i].Type {
						case jscan.TokenTypeNull:
							sl[i] = 0
						case jscan.TokenTypeInteger:
							v, overflow := atoi.I64(s[tokens[i].Index:tokens[i].End])
							if overflow {
								err = ErrorDecode{
									Err:   ErrIntegerOverflow,
									Index: tokens[i].Index,
								}
								return true
							}
							sl[i] = v
						default:
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[i].Index,
							}
							return true
						}
					}
					ti += len(tokens) + 2
					*(*[]int64)(p) = sl
					goto ON_VAL_END

				case ExpectTypeSliceUint:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					elems := tokens[ti].Elements
					if elems < 1 {
						*(*[]uint)(p) = []uint{}
						ti += 2
						goto ON_VAL_END
					}

					sl := *(*[]uint)(p)
					if cap(sl) < elems {
						sl = make([]uint, elems)
					} else {
						sl = sl[:elems]
					}

					tokens := tokens[ti+1 : tokens[ti].End]
					for i := range tokens {
						switch tokens[i].Type {
						case jscan.TokenTypeNull:
							sl[i] = 0
						case jscan.TokenTypeInteger:
							if s[tokens[i].Index] == '-' {
								err = ErrorDecode{
									Err:   ErrUnexpectedValue,
									Index: tokens[i].Index,
								}
								return true
							}
							v, errParse := d.parseUint(s[tokens[i].Index:tokens[i].End])
							if errParse != nil {
								err = ErrorDecode{
									Err:   errParse,
									Index: tokens[i].Index,
								}
								return true
							}
							sl[i] = v
						default:
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[i].Index,
							}
							return true
						}
					}
					ti += len(tokens) + 2
					*(*[]uint)(p) = sl
					goto ON_VAL_END

				case ExpectTypeSliceUint8:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					elems := tokens[ti].Elements
					if elems < 1 {
						*(*[]uint8)(p) = []uint8{}
						ti += 2
						goto ON_VAL_END
					}

					sl := *(*[]uint8)(p)
					if cap(sl) < elems {
						sl = make([]uint8, elems)
					} else {
						sl = sl[:elems]
					}

					tokens := tokens[ti+1 : tokens[ti].End]
					for i := range tokens {
						switch tokens[i].Type {
						case jscan.TokenTypeNull:
							sl[i] = 0
						case jscan.TokenTypeInteger:
							if s[tokens[i].Index] == '-' {
								err = ErrorDecode{
									Err:   ErrUnexpectedValue,
									Index: tokens[i].Index,
								}
								return true
							}
							v, overflow := atoi.U8(s[tokens[i].Index:tokens[i].End])
							if overflow {
								err = ErrorDecode{
									Err:   ErrIntegerOverflow,
									Index: tokens[i].Index,
								}
								return true
							}
							sl[i] = v
						default:
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[i].Index,
							}
							return true
						}
					}
					ti += len(tokens) + 2
					*(*[]uint8)(p) = sl
					goto ON_VAL_END

				case ExpectTypeSliceUint16:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					elems := tokens[ti].Elements
					if elems < 1 {
						*(*[]uint16)(p) = []uint16{}
						ti += 2
						goto ON_VAL_END
					}

					sl := *(*[]uint16)(p)
					if cap(sl) < elems {
						sl = make([]uint16, elems)
					} else {
						sl = sl[:elems]
					}

					tokens := tokens[ti+1 : tokens[ti].End]
					for i := range tokens {
						switch tokens[i].Type {
						case jscan.TokenTypeNull:
							sl[i] = 0
						case jscan.TokenTypeInteger:
							if s[tokens[i].Index] == '-' {
								err = ErrorDecode{
									Err:   ErrUnexpectedValue,
									Index: tokens[i].Index,
								}
								return true
							}
							v, overflow := atoi.U16(s[tokens[i].Index:tokens[i].End])
							if overflow {
								err = ErrorDecode{
									Err:   ErrIntegerOverflow,
									Index: tokens[i].Index,
								}
								return true
							}
							sl[i] = v
						default:
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[i].Index,
							}
							return true
						}
					}
					ti += len(tokens) + 2
					*(*[]uint16)(p) = sl
					goto ON_VAL_END

				case ExpectTypeSliceUint32:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					elems := tokens[ti].Elements
					if elems < 1 {
						*(*[]uint32)(p) = []uint32{}
						ti += 2
						goto ON_VAL_END
					}

					sl := *(*[]uint32)(p)
					if cap(sl) < elems {
						sl = make([]uint32, elems)
					} else {
						sl = sl[:elems]
					}

					tokens := tokens[ti+1 : tokens[ti].End]
					for i := range tokens {
						switch tokens[i].Type {
						case jscan.TokenTypeNull:
							sl[i] = 0
						case jscan.TokenTypeInteger:
							if s[tokens[i].Index] == '-' {
								err = ErrorDecode{
									Err:   ErrUnexpectedValue,
									Index: tokens[i].Index,
								}
								return true
							}
							v, overflow := atoi.U32(s[tokens[i].Index:tokens[i].End])
							if overflow {
								err = ErrorDecode{
									Err:   ErrIntegerOverflow,
									Index: tokens[i].Index,
								}
								return true
							}
							sl[i] = v
						default:
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[i].Index,
							}
							return true
						}
					}
					ti += len(tokens) + 2
					*(*[]uint32)(p) = sl
					goto ON_VAL_END

				case ExpectTypeSliceUint64:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					elems := tokens[ti].Elements
					if elems < 1 {
						*(*[]uint64)(p) = []uint64{}
						ti += 2
						goto ON_VAL_END
					}

					sl := *(*[]uint64)(p)
					if cap(sl) < elems {
						sl = make([]uint64, elems)
					} else {
						sl = sl[:elems]
					}

					tokens := tokens[ti+1 : tokens[ti].End]
					for i := range tokens {
						switch tokens[i].Type {
						case jscan.TokenTypeNull:
							sl[i] = 0
						case jscan.TokenTypeInteger:
							if s[tokens[i].Index] == '-' {
								err = ErrorDecode{
									Err:   ErrUnexpectedValue,
									Index: tokens[i].Index,
								}
								return true
							}
							v, overflow := atoi.U64(s[tokens[i].Index:tokens[i].End])
							if overflow {
								err = ErrorDecode{
									Err:   ErrIntegerOverflow,
									Index: tokens[i].Index,
								}
								return true
							}
							sl[i] = v
						default:
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[i].Index,
							}
							return true
						}
					}
					ti += len(tokens) + 2
					*(*[]uint64)(p) = sl
					goto ON_VAL_END

				case ExpectTypeSliceFloat32:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					elems := tokens[ti].Elements
					if elems < 1 {
						*(*[]float32)(p) = []float32{}
						ti += 2
						goto ON_VAL_END
					}

					sl := *(*[]float32)(p)
					if cap(sl) < elems {
						sl = make([]float32, elems)
					} else {
						sl = sl[:elems]
					}

					tokens := tokens[ti+1 : tokens[ti].End]
					for i := range tokens {
						switch tokens[i].Type {
						case jscan.TokenTypeNull:
							sl[i] = 0
						case jscan.TokenTypeNumber:
							v, errParse := d.parseFloat32(
								s[tokens[i].Index:tokens[i].End],
							)
							if errParse != nil {
								err = ErrorDecode{Err: errParse, Index: tokens[i].Index}
								return true
							}
							sl[i] = v
						case jscan.TokenTypeInteger:
							if tokens[i].End-tokens[i].Index < len("16777216") {
								// Numbers below this length are guaranteed to be smaller
								// 1<<24 and float32(i32) is faster than parseFloat32.
								i32, errParse := tokens[i].Int32(s)
								if errParse != nil {
									err = ErrorDecode{
										Err: errParse, Index: tokens[i].Index,
									}
									return true
								}
								sl[i] = float32(i32)
							} else {
								v, errParse := d.parseFloat32(
									s[tokens[i].Index:tokens[i].End],
								)
								if errParse != nil {
									err = ErrorDecode{
										Err: errParse, Index: tokens[i].Index,
									}
									return true
								}
								sl[i] = v
							}
						default:
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[i].Index,
							}
							return true
						}
					}
					ti += len(tokens) + 2
					*(*[]float32)(p) = sl
					goto ON_VAL_END

				case ExpectTypeSliceFloat64:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					elems := tokens[ti].Elements
					if elems < 1 {
						*(*[]float64)(p) = []float64{}
						ti += 2
						goto ON_VAL_END
					}

					sl := *(*[]float64)(p)
					if cap(sl) < elems {
						sl = make([]float64, elems)
					} else {
						sl = sl[:elems]
					}

					tokens := tokens[ti+1 : tokens[ti].End]
					for i := range tokens {
						switch tokens[i].Type {
						case jscan.TokenTypeNull:
							sl[i] = 0
						case jscan.TokenTypeNumber:
							v, errParse := d.parseFloat64(
								s[tokens[i].Index:tokens[i].End],
							)
							if errParse != nil {
								err = ErrorDecode{Err: errParse, Index: tokens[i].Index}
								return true
							}
							sl[i] = v
						case jscan.TokenTypeInteger:
							if tokens[i].End-tokens[i].Index < len("9007199254740992") {
								// Numbers below this length are guaranteed to be smaller
								// 1<<53a and float64(i64) is faster than parseFloat64.
								v, errParse := tokens[i].Int64(s)
								if errParse != nil {
									err = ErrorDecode{
										Err: errParse, Index: tokens[i].Index,
									}
									return true
								}
								sl[i] = float64(v)
							} else {
								v, errParse := d.parseFloat64(
									s[tokens[i].Index:tokens[i].End],
								)
								if errParse != nil {
									err = ErrorDecode{
										Err: errParse, Index: tokens[i].Index,
									}
									return true
								}
								sl[i] = v
							}
						default:
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[i].Index,
							}
							return true
						}
					}
					ti += len(tokens) + 2
					*(*[]float64)(p) = sl
					goto ON_VAL_END

				default:
					err = ErrorDecode{
						Err:   ErrUnexpectedValue,
						Index: tokens[ti].Index,
					}
					return true
				}

			case jscan.TokenTypeObject:
				switch d.stackExp[si].Type {
				case ExpectTypeEmptyStruct:
					ti = tokens[ti].End // Skip object value
					goto ON_VAL_END
				case ExpectTypeAny:
					v, tail, errDecode := decodeAny(s, tokens[ti:])
					if errDecode != nil {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					**(**interface{})(unsafe.Pointer(&p)) = v
					ti = len(tokens) - len(tail)
					goto ON_VAL_END

				case ExpectTypeMapRecur:
					// Push recursion stack
					recursiveFrame := d.stackExp[si].CapOrRecurFrame
					// fmt.Printf("PUSH STACK %p OFFSET %d SI %d\n",
					// 	d.stackExp[recursiveFrame].Dest,
					// 	d.stackExp[recursiveFrame].Offset,
					// 	si)
					d.stackExp[recursiveFrame].RecursionStack = append(
						d.stackExp[recursiveFrame].RecursionStack, recursionStackFrame{
							Dest:           d.stackExp[recursiveFrame].Dest,
							Offset:         d.stackExp[recursiveFrame].Offset,
							ContainerFrame: si,
						},
					)
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					if *(*unsafe.Pointer)(p) == nil {
						// Map not yet initialized, initialize map.
						*(*unsafe.Pointer)(p) = makemap(
							d.stackExp[si].MapType, tokens[ti].Elements,
						)
						// fmt.Printf("INIT MAP %p TO %p OFFSET %d\n",
						// 	*(*unsafe.Pointer)(p), d.stackExp[si].Dest, d.stackExp[si].Offset)
					}
					if tokens[ti].Elements == 0 {
						ti += 2
						goto ON_VAL_END
					}
					ti++

				case ExpectTypeMap:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					if *(*unsafe.Pointer)(p) == nil {
						// Map not yet initialized, initialize map.
						*(*unsafe.Pointer)(p) = makemap(
							d.stackExp[si].MapType, tokens[ti].Elements,
						)
					}
					if tokens[ti].Elements == 0 {
						ti += 2
						goto ON_VAL_END
					}
					ti++

				case ExpectTypeMapStringString:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					if tokens[ti].Elements == 0 {
						ti += 2
						*(*map[string]string)(p) = make(map[string]string, 0)
						goto ON_VAL_END
					}
					m := make(map[string]string, tokens[ti].Elements)
					tiEnd := tokens[ti].End

					for ti++; ti < tiEnd; ti += 2 {
						tokVal := tokens[ti+1]
						if tokVal.Type != jscan.TokenTypeString {
							if tokVal.Type == jscan.TokenTypeNull {
								key := s[tokens[ti].Index+1 : tokens[ti].End-1]
								keyUnescaped := unescape.Valid[S, string](key)
								m[keyUnescaped] = ""
								continue
							}
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokVal.Index,
							}
							return true
						}
						key := s[tokens[ti].Index+1 : tokens[ti].End-1]
						keyUnescaped := unescape.Valid[S, string](key)
						value := s[tokVal.Index+1 : tokVal.End-1]
						m[keyUnescaped] = unescape.Valid[S, string](value)
					}
					ti++
					*(*map[string]string)(p) = m
					goto ON_VAL_END

				case ExpectTypeStruct:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)

					if tokens[ti].Elements == 0 {
						ti += 2
						goto ON_VAL_END
					}
					// Point all fields to the struct, the offsets are already
					// set statically during decoder init time.
					for i := range d.stackExp[si].Fields {
						d.stackExp[d.stackExp[si].Fields[i].FrameIndex].Dest = p
					}
					ti++

				case ExpectTypeStructRecur:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					if tokens[ti].Elements == 0 {
						ti += 2
						goto ON_RECUR_OBJ_END
					}
					// Point all fields to the struct, the offsets are already
					// set statically during decoder init time.
					for i := range d.stackExp[si].Fields {
						d.stackExp[d.stackExp[si].Fields[i].FrameIndex].Dest = p
					}
					ti++

				default:
					err = ErrorDecode{
						Err:   ErrUnexpectedValue,
						Index: tokens[ti].Index,
					}
					return true
				}

			case jscan.TokenTypeKey:
				switch d.stackExp[si].Type {
				case ExpectTypeStruct, ExpectTypeStructRecur:
				SCAN_KEYVALS:
					for {
						key := unescape.Valid[S, string](
							s[tokens[ti].Index+1 : tokens[ti].End-1],
						)
						frameIndex := fieldFrameIndexByName(d.stackExp[si].Fields, key)
						if frameIndex == noParentFrame {
							if options.DisallowUnknownFields {
								err = ErrorDecode{
									Err:   ErrUnknownField,
									Index: tokens[ti].Index,
								}
								return true
							}
							// Skip value, go to the next key
							ti++
							for l := 0; ; ti++ {
								switch tokens[ti].Type {
								case jscan.TokenTypeKey:
									if l < 1 {
										continue SCAN_KEYVALS
									}
								case jscan.TokenTypeObject:
									l++
								case jscan.TokenTypeObjectEnd:
									l--
									if l < 1 {
										ti--
										break SCAN_KEYVALS
									}
								}
							}
						}
						si = frameIndex
						if si == noParentFrame {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						break
					}

				case ExpectTypeMap, ExpectTypeMapRecur:
					// Insert the key and assign the value frame destination pointer.

					key := s[tokens[ti].Index+1 : tokens[ti].End-1]

					// fmt.Printf("KEY %q SI %d ON %p\n", key, si, d.stackExp[si].Dest)

					typMap := d.stackExp[si].MapType
					typVal := d.stackExp[si].MapValueType
					pMap := *(*unsafe.Pointer)(unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					))
					// fmt.Printf("SOURCE MAP %p FROM %p\n", pMap, d.stackExp[si].Dest)

					// pNewData will point to the new data cell in the map.
					var pNewData unsafe.Pointer

					// The key frame is guaranteed to be non-composite (single frame)
					// and at an offset of 1 relative to the map frame index.
					switch d.stackExp[si+1].Type {
					case ExpectTypeTextUnmarshaler:
						v := reflect.New(d.stackExp[si+1].RType)
						iface := v.Interface().(encoding.TextUnmarshaler)
						keyUnescaped := unescape.Valid[S, []byte](key)
						if errU := iface.UnmarshalText(keyUnescaped); errU != nil {
							err = ErrorDecode{
								Err:   errU,
								Index: tokens[ti].Index,
							}
							return true
						}
						pKey := v.UnsafePointer()
						pNewData = mapassign(typMap, pMap, noescape(pKey))

					case ExpectTypeStr:
						keyStr := unescape.Valid[S, string](key)
						if d.stackExp[si].MapCanUseAssignFaststr {
							// fmt.Println("pMap:", pMap)
							pNewData = mapassign_faststr(typMap, pMap, keyStr)
							// fmt.Printf("ASSIGFAST %q IS NOW CELL %p\n", keyStr, pNewData)
						} else {
							pNewData = mapassign(typMap, pMap, unsafe.Pointer(&keyStr))
							// fmt.Printf("ASSIG %q IS NOW CELL %p\n", keyStr, pNewData)
						}

					case ExpectTypeInt:
						_, rc := jsonnum.ReadNumber(key)
						if rc != jsonnum.ReturnCodeInteger {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						v, errParse := d.parseInt(key)
						if errParse != nil {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						pNewData = mapassign(typMap, pMap, noescape(unsafe.Pointer(&v)))

					case ExpectTypeInt8:
						_, rc := jsonnum.ReadNumber(key)
						if rc != jsonnum.ReturnCodeInteger {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						v, overflow := atoi.I8(key)
						if overflow {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						pNewData = mapassign(typMap, pMap, noescape(unsafe.Pointer(&v)))

					case ExpectTypeInt16:
						_, rc := jsonnum.ReadNumber(key)
						if rc != jsonnum.ReturnCodeInteger {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						v, overflow := atoi.I16(key)
						if overflow {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						pNewData = mapassign(typMap, pMap, noescape(unsafe.Pointer(&v)))

					case ExpectTypeInt32:
						_, rc := jsonnum.ReadNumber(key)
						if rc != jsonnum.ReturnCodeInteger {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						v, overflow := atoi.I32(key)
						if overflow {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						pNewData = mapassign(typMap, pMap, noescape(unsafe.Pointer(&v)))

					case ExpectTypeInt64:
						_, rc := jsonnum.ReadNumber(key)
						if rc != jsonnum.ReturnCodeInteger {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						v, overflow := atoi.I64(key)
						if overflow {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						pNewData = mapassign(typMap, pMap, noescape(unsafe.Pointer(&v)))

					case ExpectTypeUint:
						_, rc := jsonnum.ReadNumber(key)
						if rc != jsonnum.ReturnCodeInteger || key[0] == '-' {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						v, errParse := d.parseUint(key)
						if errParse != nil {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						pNewData = mapassign(typMap, pMap, noescape(unsafe.Pointer(&v)))

					case ExpectTypeUint8:
						_, rc := jsonnum.ReadNumber(key)
						if rc != jsonnum.ReturnCodeInteger || key[0] == '-' {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						v, overflow := atoi.U8(key)
						if overflow {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						pNewData = mapassign(typMap, pMap, noescape(unsafe.Pointer(&v)))

					case ExpectTypeUint16:
						_, rc := jsonnum.ReadNumber(key)
						if rc != jsonnum.ReturnCodeInteger || key[0] == '-' {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						v, overflow := atoi.U16(key)
						if overflow {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						pNewData = mapassign(typMap, pMap, noescape(unsafe.Pointer(&v)))

					case ExpectTypeUint32:
						_, rc := jsonnum.ReadNumber(key)
						if rc != jsonnum.ReturnCodeInteger || key[0] == '-' {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						v, overflow := atoi.U32(key)
						if overflow {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						pNewData = mapassign(typMap, pMap, noescape(unsafe.Pointer(&v)))

					case ExpectTypeUint64:
						_, rc := jsonnum.ReadNumber(key)
						if rc != jsonnum.ReturnCodeInteger || key[0] == '-' {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						v, overflow := atoi.U64(key)
						if overflow {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						pNewData = mapassign(typMap, pMap, noescape(unsafe.Pointer(&v)))
					}

					// Point the value stack frame to the newly allocated map cell.
					// d.stackExp[si+1].Dest = pNewData

					if d.stackExp[si].Type == ExpectTypeMapRecur {
						recursiveFrame := d.stackExp[si].CapOrRecurFrame
						// // Push recursion stack before moving back to the recursive frame.
						// d.stackExp[recursiveFrame].RecursionStack = append(
						// 	d.stackExp[recursiveFrame].RecursionStack,
						// 	recursionStackFrame{
						// 		Dest:           d.stackExp[recursiveFrame].Dest,
						// 		Offset:         d.stackExp[recursiveFrame].Offset,
						// 		ContainerFrame: si,
						// 	},
						// )
						si = uint32(recursiveFrame)
					} else {
						// For non-recursive maps the value frame is guaranteed to
						// be at an offset of 2 relative to the map frame index.
						si += 2
					}

					d.stackExp[si].Dest = pNewData

					// Zero struct values
					if d.stackExp[si].Type == ExpectTypeStruct {
						typedmemclr(typVal, pNewData)
					}

				default:
					err = ErrorDecode{
						Err:   ErrUnexpectedValue,
						Index: tokens[ti].Index,
					}
					return true
				}
				ti++

			case jscan.TokenTypeObjectEnd:
				// fmt.Printf("OBJ END TI %d SI %d\n", ti, si)
				ti++
				switch d.stackExp[si].Type {
				case ExpectTypeStruct:
					goto ON_VAL_END
				case ExpectTypeMapRecur:
					recursiveFrame := d.stackExp[si].CapOrRecurFrame
					// fmt.Printf("RECURSTACK_LEN %d SI %d REC SI %d\n",
					// 	len(d.stackExp[recursiveFrame].RecursionStack), si, recursiveFrame)
					recurStack := d.stackExp[recursiveFrame].RecursionStack
					if len(recurStack) < 1 {
						// In map of the root recursive struct
						// si = uint32(d.stackExp[si].CapOrRecurFrame)
						// fmt.Printf("%d\tIN ROOT MAP\n", ti)
						goto ON_VAL_END
					}

					si = uint32(d.stackExp[si].ParentFrameIndex)

					// Reset to parent context
					// dumpStack("BEFORE RESET")
					topIndex := len(recurStack) - 1
					resetTo := d.stackExp[si].RecursionStack[topIndex]
					d.stackExp[si].Dest = resetTo.Dest
					d.stackExp[si].Offset = resetTo.Offset
					fields := d.stackExp[si].Fields
					for i := range fields {
						d.stackExp[fields[i].FrameIndex].Dest = resetTo.Dest
					}

					// Pop recursion stack
					recurStack[topIndex].Dest = nil
					d.stackExp[si].RecursionStack = recurStack[:topIndex]
					// fmt.Println("EXITED RECURSIVE MAP", si)
					// dumpStack("AFTER RESET")
					continue
				}
				goto ON_RECUR_OBJ_END

			case jscan.TokenTypeArrayEnd:
				ti++
				if d.stackExp[si].Type == ExpectTypeStructRecur {
					recurStack := d.stackExp[si].RecursionStack
					if len(recurStack) < 1 {
						// In slice of the root recursive struct
						// si = uint32(d.stackExp[si].CapOrRecurFrame)
						// fmt.Printf("%d\tIN ROOT\n", ti)
						goto ON_VAL_END
					}

					// Reset to parent context
					// dumpStack("BEFORE RESET")
					topIndex := len(recurStack) - 1
					resetTo := d.stackExp[si].RecursionStack[topIndex]
					// fmt.Printf("%d\tRESET SI %d\tTO %p OFFSET %d AND MOVE TO %d\n",
					// 	ti, si, resetTo.Dest, resetTo.Offset, resetTo.ContainerFrame)
					d.stackExp[si].Dest = resetTo.Dest
					d.stackExp[si].Offset = resetTo.Offset

					// Pop recursion stack
					recurStack[topIndex].Dest = nil
					d.stackExp[si].RecursionStack = recurStack[:topIndex]
					si = d.stackExp[resetTo.ContainerFrame].ParentFrameIndex
					// dumpStack("AFTER RESET")
					continue
				}
				si--
				goto ON_VAL_END
			}
			continue

		ON_RECUR_OBJ_END:
			if l := len(d.stackExp[si].RecursionStack); l > 0 {
				siCon := d.stackExp[si].RecursionStack[l-1].ContainerFrame
				// fmt.Printf("%d\tRECOBJ END\tSI %d CONTAINER %d\n",
				// 	ti, si, siCon)
				switch d.stackExp[siCon].Type {
				case ExpectTypeSliceRecur:
					d.stackExp[si].Offset += d.stackExp[si].Size
				case ExpectTypeMapRecur:
					si = siCon
					// fmt.Println("BACK TO RECUR MAP", si)
				}
				continue
			}
			// Here we expect to fallthrough directly to ON_VAL_END!

		ON_VAL_END:
			if siCon := d.stackExp[si].ParentFrameIndex; siCon != noParentFrame {
				switch d.stackExp[siCon].Type {
				case ExpectTypePtr:
					si--
				case ExpectTypeArray:
					d.stackExp[si].Len++
					if d.stackExp[si].Len >= d.stackExp[si].CapOrRecurFrame {
						// Skip all extra values
					SKIP_ALL_EXTRA_VALUES:
						for l := 1; ; ti++ {
							switch tokens[ti].Type {
							case jscan.TokenTypeArray:
								l++
							case jscan.TokenTypeArrayEnd:
								l--
								if l < 1 {
									break SKIP_ALL_EXTRA_VALUES
								}
							}
						}
					} else {
						d.stackExp[si].Offset += d.stackExp[si].Size
					}
				case ExpectTypeSlice:
					d.stackExp[si].Offset += d.stackExp[si].Size

				case ExpectTypeMap, ExpectTypeStruct, ExpectTypeStructRecur:
					si = siCon
					if si == noParentFrame {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
				}
			}
			continue

		ON_JSON_UNMARSHALER:
			{
				p := unsafe.Pointer(
					uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
				)
				var raw S
				tkIndex := tokens[ti].Index
				switch tokens[ti].Type {
				case jscan.TokenTypeObject, jscan.TokenTypeArray:
					// Composite value
					raw = s[tokens[ti].Index : tokens[tokens[ti].End].Index+1]
					ti = tokens[ti].End + 1
				default:
					// Non-composite value
					raw = s[tokens[ti].Index:tokens[ti].End]
					ti++
				}
				u := reflect.NewAt(d.stackExp[si].RType, p).Interface().(json.Unmarshaler)
				if errUnmarshal := u.UnmarshalJSON([]byte(raw)); errUnmarshal != nil {
					err = ErrorDecode{
						Err:   errUnmarshal,
						Index: tkIndex,
					}
					return true
				}
				goto ON_VAL_END
			}
		}
		return false
	})
	if errTok.IsErr() {
		if errTok.Code == jscan.ErrorCodeCallback {
			return err
		}
		return ErrorDecode{
			Err:   errTok,
			Index: errTok.Index,
		}
	}
	*t = *(*T)(d.stackExp[0].Dest)
	// fmt.Println("FU", d.stackExp[0].Dest)
	return ErrorDecode{}
}

func allocate(n uintptr) unsafe.Pointer { return unsafe.Pointer(&make([]byte, n)[0]) }

type sliceHeader struct {
	Data     unsafe.Pointer
	Len, Cap uintptr
}

var (
	zeroBool    bool
	zeroStr     string
	zeroInt     int
	zeroUint    uint
	zeroInt8    int8
	zeroUint8   uint8
	zeroInt16   int16
	zeroUint16  uint16
	zeroInt32   int32
	zeroUint32  uint32
	zeroInt64   int64
	zeroUint64  uint64
	zeroFloat32 float32
	zeroFloat64 float64
)

// fieldFrameIndexByName returns the frame index of the field identified by name
// or -1 if no field is found. Exact matches are prioritized over
// case-insensitive matches.
func fieldFrameIndexByName[S []byte | string](fields []fieldStackFrame, name S) uint32 {
	{ // Check for exact matches first
		f := fields
		for ; len(f) >= 4; f = f[4:] { // Try 4 at a time if enough are left
			if string(f[0].Name) == string(name) {
				return f[0].FrameIndex
			}
			if string(f[1].Name) == string(name) {
				return f[1].FrameIndex
			}
			if string(f[2].Name) == string(name) {
				return f[2].FrameIndex
			}
			if string(f[3].Name) == string(name) {
				return f[3].FrameIndex
			}
		}
		for i := range f {
			if string(f[i].Name) == string(name) {
				return f[i].FrameIndex
			}
		}
	}
	// Fallback to case-insensitive search
	f := fields
	eq := strings.EqualFold
	for ; len(f) >= 4; f = f[4:] { // Try 4 at a time if enough are left
		if len(f[0].Name) == len(name) && eq(string(f[0].Name), string(name)) {
			return f[0].FrameIndex
		}
		if len(f[1].Name) == len(name) && eq(string(f[1].Name), string(name)) {
			return f[1].FrameIndex
		}
		if len(f[2].Name) == len(name) && eq(string(f[2].Name), string(name)) {
			return f[2].FrameIndex
		}
		if len(f[3].Name) == len(name) && eq(string(f[3].Name), string(name)) {
			return f[3].FrameIndex
		}
	}
	for i := range f {
		if len(f[i].Name) == len(name) && eq(string(f[i].Name), string(name)) {
			return f[i].FrameIndex
		}
	}
	return noParentFrame
}

func decodeAny[S ~[]byte | ~string](
	str S, tokens []jscan.Token[S],
) (any, []jscan.Token[S], error) {
	switch tokens[0].Type {
	case jscan.TokenTypeNull:
		return nil, tokens[1:], nil
	case jscan.TokenTypeNumber, jscan.TokenTypeInteger:
		f64, err := tokens[0].Float64(str)
		if err != nil {
			return nil, nil, err
		}
		return f64, tokens[1:], nil
	case jscan.TokenTypeTrue:
		return true, tokens[1:], nil
	case jscan.TokenTypeFalse:
		return false, tokens[1:], nil
	case jscan.TokenTypeString:
		return unescape.Valid[S, string](
			str[tokens[0].Index+1 : tokens[0].End-1],
		), tokens[1:], nil
	case jscan.TokenTypeArray:
		l := make([]any, 0, tokens[0].Elements)
		for tokens = tokens[1:]; tokens[0].Type != jscan.TokenTypeArrayEnd; {
			var v any
			var err error
			if v, tokens, err = decodeAny(str, tokens); err != nil {
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
			if v, tokens, err = decodeAny(str, tokens[1:]); err != nil {
				return nil, nil, err
			}
			m[unescape.Valid[S, string](key)] = v
		}
		return m, tokens[1:], nil
	}
	panic("unreachable")
}

// emptyStructAddr the address to an empty slice remains the same
// throughout the life of the process.
var emptyStructAddr = unsafe.Pointer(&struct{}{})

//go:linkname noescape reflect.noescape
func noescape(p unsafe.Pointer) unsafe.Pointer

//go:linkname typedmemclr runtime.typedmemclr
//go:noescape
func typedmemclr(typ *typ, dst unsafe.Pointer)

//go:linkname typedmemmove reflect.typedmemmove
func typedmemmove(t *typ, dst, src unsafe.Pointer)

//go:linkname makemap reflect.makemap
func makemap(*typ, int) unsafe.Pointer

//nolint:golint
//go:linkname mapassign_faststr runtime.mapassign_faststr
//go:noescape
func mapassign_faststr(t *typ, m unsafe.Pointer, s string) unsafe.Pointer

//go:linkname mapassign runtime.mapassign
//go:noescape
func mapassign(t *typ, m unsafe.Pointer, k unsafe.Pointer) unsafe.Pointer

// typ represents reflect.rtype for noescape trick
type typ struct{}

type emptyInterface struct {
	_   *typ
	ptr unsafe.Pointer
}

func getTyp(t reflect.Type) *typ {
	return (*typ)(((*emptyInterface)(unsafe.Pointer(&t))).ptr)
}

func canUseAssignFaststr(mapType reflect.Type) bool {
	return mapType.Elem().Size() <= 128 && mapType.Key().Kind() == reflect.String
}
