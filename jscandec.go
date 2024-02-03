package jscandec

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
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

	ExpectTypeJSONUnmarshaler
	ExpectTypeTextUnmarshaler
	ExpectTypePtr
	ExpectTypeAny
	ExpectTypeMap
	ExpectTypeArray
	ExpectTypeSlice
	ExpectTypeStruct
	ExpectTypeEmptyStruct
	ExpectTypeBool
	ExpectTypeStr
	ExpectTypeFloat32
	ExpectTypeFloat64
	ExpectTypeInt
	ExpectTypeInt8
	ExpectTypeInt16
	ExpectTypeInt32
	ExpectTypeInt64
	ExpectTypeUint
	ExpectTypeUint8
	ExpectTypeUint16
	ExpectTypeUint32
	ExpectTypeUint64
)

func (t ExpectType) String() string {
	switch t {
	case ExpectTypeJSONUnmarshaler:
		return "interface{UnmarshalJSON([]byte)error}"
	case ExpectTypeTextUnmarshaler:
		return "interface{UnmarshalText([]byte)error}"
	case ExpectTypePtr:
		return "*"
	case ExpectTypeAny:
		return "any"
	case ExpectTypeMap:
		return "map"
	case ExpectTypeArray:
		return "array"
	case ExpectTypeSlice:
		return "slice"
	case ExpectTypeStruct:
		return "struct"
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
	}
	return ""
}

// fieldStackFrame identifies a field within a struct frame.
type fieldStackFrame struct {
	// FrameIndex defines the stack index of the field's value frame.
	FrameIndex int

	// Name defines either the name of the field in the struct
	// or the json struct tag if any.
	Name string
}

type stackFrame[S []byte | string] struct {
	// Fields is only relevant to structs.
	// For every other type Fields is always nil.
	Fields []fieldStackFrame

	// RType is relevant to map and JSONUnmarshaler frames only.
	RType reflect.Type

	// LastMapKey is relevant to map frames only.
	LastMapKey any

	// MapValue is relevant to map frames only.
	// MapValue is set at runtime and must be reset on every call to Decode
	// to avoid keeping an unsafe.Pointer to the allocated map and allow the GC
	// to clean it up if necessary.
	MapValue reflect.Value

	// Size defines the number of bytes the data would occupy in memory,
	// except for arrays, for which it defines the static length of the array.
	// Size could be taken from reflect.Type but it's slower than storing it here.
	Size uintptr

	// Type defines what data type is expected at this frame.
	// Same as Size, Type kind could be taken from reflect.Type but it's
	// slower than storing it here.
	Type ExpectType

	// Dest defines the destination memory to write the data to.
	// Dest is set at runtime and must be reset on every call to Decode
	// to avoid keeping a pointer to the allocated data and allow the GC
	// to clean it up if necessary.
	Dest unsafe.Pointer // Overwritten at runtime

	// Offset is used at runtime for pointer arithmetics if the decoder is
	// currently inside of an array or slice.
	// For struct fields however, Offset is assigned statically at decoder init time.
	Offset uintptr // Overwritten at runtime

	// AdvanceBy is used at runtime for pointer arithmetics to define
	// how much to advance the pointer by.
	// AdvanceBy=0x0 means we're not inside of an array or slice.
	AdvanceBy uintptr

	// ParentFrameIndex defines the index of the composite parent object in the stack.
	ParentFrameIndex int

	// OptionString defines whether the `json:",string"` option was specified.
	OptionString bool // TODO: implement support
}

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
	stack = appendTypeToStack(stack, reflect.TypeOf(*t))
	tokenizer := jscan.NewTokenizer[S](
		len(stack)+1, len(s)/2,
	)
	d := Decoder[S, T]{tokenizer: tokenizer, stackExp: stack}
	if err := d.Decode(s, t, DefaultOptions); err.IsErr() {
		return err.Err
	}
	return nil
}

// Decoder is a reusable decoder instance.
type Decoder[S []byte | string, T any] struct {
	tokenizer *jscan.Tokenizer[S]
	stackExp  []stackFrame[S]
}

// NewDecoder creates a new reusable decoder instance.
// In case there are multiple decoder instances for different types of T,
// tokenizer is recommended to be shared across them, yet the decoders must
// not be used concurrently!
func NewDecoder[S []byte | string, T any](tokenizer *jscan.Tokenizer[S]) *Decoder[S, T] {
	d := &Decoder[S, T]{
		tokenizer: tokenizer,
		stackExp:  make([]stackFrame[S], 0, 4),
	}

	var z T
	d.stackExp = appendTypeToStack(d.stackExp, reflect.TypeOf(z))

	return d
}

// appendTypeToStack will recursively flat-append stack frames recursing into t.
func appendTypeToStack[S []byte | string](
	stack []stackFrame[S], t reflect.Type,
) []stackFrame[S] {
	if t == nil {
		return append(stack, stackFrame[S]{
			Type:             ExpectTypeAny,
			Size:             unsafe.Sizeof(struct{ typ, dat uintptr }{}),
			ParentFrameIndex: len(stack) - 1,
		})
	} else if s := determineJSONUnmarshalerSupport(t); s != interfaceSupportNone {
		return append(stack, stackFrame[S]{
			Type:             ExpectTypeJSONUnmarshaler,
			RType:            t,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	} else if s := determineTextUnmarshalerSupport(t); s != interfaceSupportNone {
		return append(stack, stackFrame[S]{
			Type:             ExpectTypeTextUnmarshaler,
			RType:            t,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
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
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Array:
		parentIndex := len(stack)
		stack = append(stack, stackFrame[S]{
			Size:             t.Size(),
			Type:             ExpectTypeArray,
			ParentFrameIndex: len(stack) - 1,
		})
		newAtIndex := len(stack)
		stack = appendTypeToStack(stack, t.Elem())

		// Link array element to the array frame.
		stack[newAtIndex].ParentFrameIndex = parentIndex

	case reflect.Slice:
		parentIndex := len(stack)
		stack = append(stack, stackFrame[S]{
			Size:             t.Size(),
			Type:             ExpectTypeSlice,
			ParentFrameIndex: len(stack) - 1,
		})
		newAtIndex := len(stack)
		stack = appendTypeToStack(stack, t.Elem())

		// Link slice element to the slice frame.
		stack[newAtIndex].ParentFrameIndex = parentIndex

	case reflect.Map:
		parentIndex := len(stack)
		stack = append(stack, stackFrame[S]{
			// The map will be handled via reflect.Value
			// hence the size is not t.Size().
			Size:             t.Size(),
			Type:             ExpectTypeMap,
			ParentFrameIndex: len(stack) - 1,
			RType:            t,
		})
		{
			newAtIndex := len(stack)
			stack = appendTypeToStack(stack, t.Key())
			// Link map key to the map frame.
			stack[newAtIndex].ParentFrameIndex = parentIndex
		}
		{
			newAtIndex := len(stack)
			stack = appendTypeToStack(stack, t.Elem())
			// Link map value to the map frame.
			stack[newAtIndex].ParentFrameIndex = parentIndex
		}

	case reflect.Struct:
		parentIndex := len(stack)
		numFields := t.NumField()
		if numFields == 0 {
			return append(stack, stackFrame[S]{
				Size:             t.Size(),
				Type:             ExpectTypeEmptyStruct,
				ParentFrameIndex: len(stack) - 1,
			})
		}
		stack = append(stack, stackFrame[S]{
			Fields:           make([]fieldStackFrame, 0, numFields),
			Size:             t.Size(),
			Type:             ExpectTypeStruct,
			ParentFrameIndex: len(stack) - 1,
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

			newAtIndex := len(stack)
			stack = appendTypeToStack(stack, f.Type)
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

			// Set `string` option if defined.
			stack[newAtIndex].OptionString = optionString
		}

	case reflect.Bool:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeBool,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.String:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeStr,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Int:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeInt,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Int8:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeInt8,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Int16:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeInt16,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Int32:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeInt32,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Int64:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeInt64,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Uint:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeUint,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Uint8:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeUint8,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Uint16:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeUint16,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Uint32:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeUint32,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Uint64:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeUint64,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Float32:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeFloat32,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Float64:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeFloat64,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Pointer:
		parentIndex := len(stack)
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypePtr,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
		newAtIndex := len(stack)
		stack = appendTypeToStack(stack, t.Elem())
		stack[newAtIndex].ParentFrameIndex = parentIndex

	default:
		panic(fmt.Errorf("TODO: unsupported type: %v", t))
	}
	return stack
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

type DecodeOptions struct {
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
			d.stackExp[i].MapValue = reflect.Value{}
		}
	}()

	if t == nil {
		return ErrorDecode{Err: ErrNilDest}
	}

	var fnA2I func(s S) (int, error)
	var fnA2UI func(s S) (uint, error)

	if unsafe.Sizeof(int(0)) == 8 {
		// 64-bit system
		fnA2I = func(s S) (int, error) {
			i, overflow := atoi.I64(s)
			if overflow {
				return 0, ErrIntegerOverflow
			}
			return int(i), nil
		}
		fnA2UI = func(s S) (uint, error) {
			i, overflow := atoi.U64(s)
			if overflow {
				return 0, ErrIntegerOverflow
			}
			return uint(i), nil
		}
	} else {
		// 32-bit system
		fnA2I = func(s S) (int, error) {
			i, overflow := atoi.I32(s)
			if overflow {
				return 0, ErrIntegerOverflow
			}
			return int(i), nil
		}
		fnA2UI = func(s S) (uint, error) {
			i, overflow := atoi.U32(s)
			if overflow {
				return 0, ErrIntegerOverflow
			}
			return uint(i), nil
		}
	}

	fnA2F32 := func(s S, dest unsafe.Pointer) error {
		v, err := strconv.ParseFloat(string(s), 32)
		if err != nil {
			return err
		}
		*(*float32)(dest) = float32(v)
		return nil
	}
	fnA2F64 := func(s S, dest unsafe.Pointer) error {
		v, err := strconv.ParseFloat(string(s), 64)
		if err != nil {
			return err
		}
		*(*float64)(dest) = v
		return nil
	}
	var sz S
	if _, ok := any(sz).([]byte); ok {
		// Avoid copying the slice data
		fnA2F32 = func(s S, dest unsafe.Pointer) error {
			su := unsafe.String(unsafe.SliceData([]byte(s)), len(s))
			v, err := strconv.ParseFloat(su, 32)
			if err != nil {
				return err
			}
			*(*float32)(dest) = float32(v)
			return nil
		}
		fnA2F64 = func(s S, dest unsafe.Pointer) error {
			su := unsafe.String(unsafe.SliceData([]byte(s)), len(s))
			v, err := strconv.ParseFloat(su, 64)
			if err != nil {
				return err
			}
			*(*float64)(dest) = v
			return nil
		}
	}

	si := 0
	d.stackExp[0].Dest = unsafe.Pointer(t)

	dest := func() unsafe.Pointer {
		return unsafe.Pointer(uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset)
	}

	errTok := d.tokenizer.Tokenize(s, func(tokens []jscan.Token[S]) (exit bool) {
		for ti := 0; ti < len(tokens); {
			if d.stackExp[si].Type == ExpectTypePtr {
				p := dest()
				si++
				dp := allocate(d.stackExp[si].Size)
				d.stackExp[si].Dest = dp
				*(*unsafe.Pointer)(p) = dp
				continue
			}
			switch tokens[ti].Type {
			case jscan.TokenTypeFalse, jscan.TokenTypeTrue:
				switch d.stackExp[si].Type {
				case ExpectTypeJSONUnmarshaler:
					goto ON_JSON_UNMARSHALER
				case ExpectTypeAny:
					p := dest()
					v := tokens[ti].Type == jscan.TokenTypeTrue
					**(**interface{})(unsafe.Pointer(&p)) = v
				case ExpectTypeBool:
					p := dest()
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
				case ExpectTypeJSONUnmarshaler:
					goto ON_JSON_UNMARSHALER
				case ExpectTypeAny:
					p := dest()
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
							Err:   ErrUnexpectedValue,
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
					if i, e := fnA2UI(s[tokens[ti].Index:tokens[ti].End]); e != nil {
						// Invalid unsigned integer
						err = ErrorDecode{
							Err:   e,
							Index: tokens[ti].Index,
						}
						return true
					} else {
						p := dest()
						*(*uint)(p) = i
					}

				case ExpectTypeInt:
					if ui, e := fnA2I(s[tokens[ti].Index:tokens[ti].End]); e != nil {
						// Invalid signed integer
						err = ErrorDecode{
							Err:   e,
							Index: tokens[ti].Index,
						}
						return true
					} else {
						p := dest()
						*(*int)(p) = ui
					}

				case ExpectTypeUint8:
					p := dest()
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
					p := dest()
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
					p := dest()
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
					p := dest()
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
					p := dest()
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
					p := dest()
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
					p := dest()
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
					p := dest()
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
					p := dest()
					if e := fnA2F32(s[tokens[ti].Index:tokens[ti].End], p); e != nil {
						err = ErrorDecode{Err: e, Index: tokens[ti].Index}
						return true
					}

				case ExpectTypeFloat64:
					p := dest()
					if e := fnA2F64(s[tokens[ti].Index:tokens[ti].End], p); e != nil {
						err = ErrorDecode{Err: e, Index: tokens[ti].Index}
						return true
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
				case ExpectTypeJSONUnmarshaler:
					goto ON_JSON_UNMARSHALER
				case ExpectTypeAny:
					p := dest()
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
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					**(**interface{})(unsafe.Pointer(&p)) = v

				case ExpectTypeFloat32:
					p := dest()
					if e := fnA2F32(s[tokens[ti].Index:tokens[ti].End], p); e != nil {
						err = ErrorDecode{Err: e, Index: tokens[ti].Index}
						return true
					}
				case ExpectTypeFloat64:
					p := dest()
					if e := fnA2F64(s[tokens[ti].Index:tokens[ti].End], p); e != nil {
						err = ErrorDecode{Err: e, Index: tokens[ti].Index}
						return true
					}
				default:
					err = ErrorDecode{Err: ErrUnexpectedValue, Index: tokens[ti].Index}
					return true
				}
				ti++
				goto ON_VAL_END

			case jscan.TokenTypeString:
				p := dest()

				switch d.stackExp[si].Type {
				case ExpectTypeJSONUnmarshaler:
					goto ON_JSON_UNMARSHALER
				case ExpectTypeTextUnmarshaler:
					p := dest()
					u := reflect.NewAt(
						d.stackExp[si].RType, p,
					).Interface().(encoding.TextUnmarshaler)
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					tb := unescape.Valid[S, []byte](tv)
					if errUnmarshal := u.UnmarshalText(tb); errUnmarshal != nil {
						err = ErrorDecode{
							Err:   errUnmarshal,
							Index: tokens[ti].Index,
						}
						return true
					}
				case ExpectTypeAny:
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					**(**interface{})(unsafe.Pointer(&p)) = unescape.Valid[S, string](tv)
				case ExpectTypeStr:
					tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
					*(*string)(p) = unescape.Valid[S, string](tv)
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
				p := dest()
				switch d.stackExp[si].Type {
				case ExpectTypeJSONUnmarshaler:
					goto ON_JSON_UNMARSHALER
				case ExpectTypeEmptyStruct:
					// Nothing
				case ExpectTypeAny:
					// Nothing
				case ExpectTypeSlice:
					// Skip
				case ExpectTypeStruct:
					// Nothing, the struct is already zeroed
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
				}
				ti++
				goto ON_VAL_END

			case jscan.TokenTypeArray:
				switch d.stackExp[si].Type {
				case ExpectTypeJSONUnmarshaler:
					goto ON_JSON_UNMARSHALER
				case ExpectTypeAny:
					p := dest()
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

				case ExpectTypeArray:
					p := dest()
					if tokens[ti].Elements < 1 {
						ti += 2 // Skip over closing TokenArrayEnd
						// Empty array, no need to allocate memory.
						// Go will automatically allocate it together with its parent
						// and zero it.
						goto ON_VAL_END
					}
					if uintptr(tokens[ti].Elements) > d.stackExp[si].Size {
						// Go array is too small to accommodate all values.
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}

					elementSize := d.stackExp[si+1].Size

					ti++
					si++
					d.stackExp[si].Dest = p
					d.stackExp[si].Offset = 0
					d.stackExp[si].AdvanceBy = elementSize

				case ExpectTypeSlice:
					p := dest()
					elementSize := d.stackExp[si+1].Size

					var dp unsafe.Pointer
					if elems := uintptr(tokens[ti].Elements); elems == 0 {
						// Allocate empty slice
						dp = allocate(elementSize)
						*(*sliceHeader)(p) = sliceHeader{Data: dp, Len: 0, Cap: 0}
						ti += 2
						goto ON_VAL_END
					} else {
						dp = allocate(elems * elementSize)
						*(*sliceHeader)(p) = sliceHeader{Data: dp, Len: elems, Cap: elems}

						si++
						d.stackExp[si].Dest = dp
						d.stackExp[si].Offset = 0
						d.stackExp[si].AdvanceBy = elementSize
					}
					ti++

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
				case ExpectTypeJSONUnmarshaler:
					goto ON_JSON_UNMARSHALER
				case ExpectTypeAny:
					v, tail, errDecode := decodeAny(s, tokens[ti:])
					if errDecode != nil {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					p := dest()
					**(**interface{})(unsafe.Pointer(&p)) = v
					ti = len(tokens) - len(tail)
					goto ON_VAL_END

				case ExpectTypeMap:
					p := dest()
					d.stackExp[si].MapValue = reflect.MakeMapWithSize(
						d.stackExp[si].RType, tokens[ti].Elements,
					)
					*(*unsafe.Pointer)(p) = d.stackExp[si].MapValue.UnsafePointer()
					if tokens[ti].Elements == 0 {
						ti += 2
						goto ON_VAL_END
					}
					ti++

				case ExpectTypeStruct:
					p := dest()
					if tokens[ti+1].Type == jscan.TokenTypeObjectEnd {
						ti += 2
						goto ON_VAL_END
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
				case ExpectTypeStruct:
				SCAN_KEYVALS:
					for {
						tv := s[tokens[ti].Index+1 : tokens[ti].End-1]
						key := unescape.Valid[S, string](tv)
						frameIndex := fieldFrameIndexByName(d.stackExp[si].Fields, key)
						if frameIndex == -1 {
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
						if si < 0 {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						break
					}

				case ExpectTypeMap:
					// Record the key for insertion (once the value is read)
					key := s[tokens[ti].Index+1 : tokens[ti].End-1]

					// The key frame is guaranteed to be non-composite (single frame)
					// and at an offset of 1 relative to the map frame index.
					switch d.stackExp[si+1].Type {
					case ExpectTypeTextUnmarshaler:
						keyType := d.stackExp[si+1].RType
						v := reflect.New(keyType)
						iface := v.Interface().(encoding.TextUnmarshaler)
						keyUnescaped := unescape.Valid[S, []byte](key)
						if errU := iface.UnmarshalText(keyUnescaped); errU != nil {
							err = ErrorDecode{
								Err:   errU,
								Index: tokens[ti].Index,
							}
							return true
						}
						d.stackExp[si].LastMapKey = v

					case ExpectTypeStr:
						d.stackExp[si].LastMapKey = unescape.Valid[S, string](key)

					case ExpectTypeInt:
						_, rc := jsonnum.ReadNumber(key)
						if rc != jsonnum.ReturnCodeInteger {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						v, errParse := fnA2I(key)
						if errParse != nil {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						d.stackExp[si].LastMapKey = v

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
						d.stackExp[si].LastMapKey = v

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
						d.stackExp[si].LastMapKey = v

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
						d.stackExp[si].LastMapKey = v

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
						d.stackExp[si].LastMapKey = v

					case ExpectTypeUint:
						_, rc := jsonnum.ReadNumber(key)
						if rc != jsonnum.ReturnCodeInteger || key[0] == '-' {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						v, errParse := fnA2UI(key)
						if errParse != nil {
							err = ErrorDecode{
								Err:   ErrUnexpectedValue,
								Index: tokens[ti].Index,
							}
							return true
						}
						d.stackExp[si].LastMapKey = v

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
						d.stackExp[si].LastMapKey = v

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
						d.stackExp[si].LastMapKey = v

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
						d.stackExp[si].LastMapKey = v

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
						d.stackExp[si].LastMapKey = v
					}

					// The value frame is guaranteed to be at an offset of 2
					// relative to the map frame index.
					si += 2
					d.stackExp[si].Dest = allocate(d.stackExp[si].Size)

				default:
					err = ErrorDecode{
						Err:   ErrUnexpectedValue,
						Index: tokens[ti].Index,
					}
					return true
				}
				ti++

			case jscan.TokenTypeObjectEnd:
				ti++
				goto ON_VAL_END
			case jscan.TokenTypeArrayEnd:
				if tokens[tokens[ti].End].Elements != 0 {
					si--
				}
				ti++
				goto ON_VAL_END
			}
			continue

		ON_VAL_END:
			if siParent := d.stackExp[si].ParentFrameIndex; siParent > -1 {
				switch d.stackExp[siParent].Type {
				case ExpectTypePtr:
					si--
				case ExpectTypeArray, ExpectTypeSlice:
					d.stackExp[si].Offset += d.stackExp[si].AdvanceBy
				case ExpectTypeMap:
					var valKey reflect.Value
					switch d.stackExp[siParent+1].Type {
					case ExpectTypeStr:
						valKey = reflect.ValueOf(d.stackExp[siParent].LastMapKey.(string))
					case ExpectTypeTextUnmarshaler:
						valKey = d.stackExp[siParent].LastMapKey.(reflect.Value).Elem()
					case ExpectTypeInt:
						valKey = reflect.ValueOf(d.stackExp[siParent].LastMapKey.(int))
					case ExpectTypeInt8:
						valKey = reflect.ValueOf(d.stackExp[siParent].LastMapKey.(int8))
					case ExpectTypeInt16:
						valKey = reflect.ValueOf(d.stackExp[siParent].LastMapKey.(int16))
					case ExpectTypeInt32:
						valKey = reflect.ValueOf(d.stackExp[siParent].LastMapKey.(int32))
					case ExpectTypeInt64:
						valKey = reflect.ValueOf(d.stackExp[siParent].LastMapKey.(int64))
					case ExpectTypeUint:
						valKey = reflect.ValueOf(d.stackExp[siParent].LastMapKey.(uint))
					case ExpectTypeUint8:
						valKey = reflect.ValueOf(d.stackExp[siParent].LastMapKey.(uint8))
					case ExpectTypeUint16:
						valKey = reflect.ValueOf(d.stackExp[siParent].LastMapKey.(uint16))
					case ExpectTypeUint32:
						valKey = reflect.ValueOf(d.stackExp[siParent].LastMapKey.(uint32))
					case ExpectTypeUint64:
						valKey = reflect.ValueOf(d.stackExp[siParent].LastMapKey.(uint64))
					}
					// Add key-value pair to the map
					d.stackExp[siParent].MapValue.SetMapIndex(
						valKey,
						reflect.NewAt(
							d.stackExp[siParent].RType.Elem(), d.stackExp[si].Dest,
						).Elem(),
					)
					fallthrough
				case ExpectTypeStruct:
					si = siParent
					if si < 0 {
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
				p := dest()
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
			Err:   ErrUnexpectedValue,
			Index: errTok.Index,
		}
	}
	*t = *(*T)(d.stackExp[0].Dest)
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
func fieldFrameIndexByName[S []byte | string](fields []fieldStackFrame, name S) int {
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
	return -1
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
		tv := str[tokens[0].Index+1 : tokens[0].End-1]
		return unescape.Valid[S, string](tv), tokens[1:], nil
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
