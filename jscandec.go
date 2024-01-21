package jscandec

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unsafe"

	"github.com/romshark/jscan-experimental-decoder/internal/atoi"

	"github.com/romshark/jscan/v2"
)

var (
	ErrNilDest         = errors.New("decoding to nil pointer")
	ErrUnexpectedValue = errors.New("unexpected value")
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
	ExpectTypeArray
	ExpectTypeSlice
	ExpectTypeStruct
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
	case ExpectTypeArray:
		return "array"
	case ExpectTypeSlice:
		return "slice"
	case ExpectTypeStruct:
		return "struct"
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

type stackFrame struct {
	// Fields is only relevant to structs.
	// For every other type Fields is always nil.
	Fields []fieldStackFrame

	// Size defines the number of bytes the data would occupy in memory,
	// except for arrays, for which it defines the static length of the array.
	Size uintptr

	// Type defines what data type is expected at this frame.
	Type ExpectType

	// Dest defines the destination memory to write the data to.
	Dest unsafe.Pointer // Overwritten at runtime

	// Offset and AdvanceBy are used at runtime to define where to start from
	// and how much to advance by if we're currently inside of an array or slice.
	// For struct fields however, Offset is assigned statically at decoder init time.
	Offset, AdvanceBy uintptr // Overwritten at runtime

	// ParentFrameIndex defines the index of the composite parent object in the stack.
	ParentFrameIndex int

	// OptionString defines whether the `json:",string"` option was specified.
	OptionString bool // TODO: implement support
}

// Unmarshal dynamically allocates new decoder and tokenizer instances and
// unmarshals the JSON contents of s into t.
// Unmarshal is primarily a drop-in replacement for the standard encoding/json.Unmarshal
// and behaves identically except for error messages. To significantly improve
// performance by avoiding dynamic decoder and tokenizer allocation and reflection at
// runtime create a reusable decoder instance using NewDecoder.
func Unmarshal[S []byte | string, T any](s S, t *T) error {
	tokenizer := jscan.NewTokenizer[S](
		jscan.DefaultStackSizeTokenizer, len(s)/2,
	)
	decoder := NewDecoder[S, T](tokenizer)
	if err := decoder.Decode(s, t); err.IsErr() {
		return err.Err
	}
	return nil
}

// Decoder is a reusable decoder instance.
type Decoder[S []byte | string, T any] struct {
	tokenizer *jscan.Tokenizer[S]
	stackExp  []stackFrame
}

// NewDecoder creates a new reusable decoder instance.
// In case there are multiple decoder instances for different types of T,
// tokenizer is recommended to be shared across them, yet the decoders must
// not be used concurrently!
func NewDecoder[S []byte | string, T any](tokenizer *jscan.Tokenizer[S]) *Decoder[S, T] {
	d := &Decoder[S, T]{
		tokenizer: tokenizer,
		stackExp:  make([]stackFrame, 0, 4),
	}

	var z T
	d.stackExp = appendTypeToStack(d.stackExp, reflect.TypeOf(z))

	return d
}

// appendTypeToStack will recursively flat-append stack frames recursing into t.
func appendTypeToStack(stack []stackFrame, t reflect.Type) []stackFrame {
	switch t.Kind() {
	case reflect.Array:
		parentIndex := len(stack)
		stack = append(stack, stackFrame{
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
		stack = append(stack, stackFrame{
			Size:             t.Size(),
			Type:             ExpectTypeSlice,
			ParentFrameIndex: len(stack) - 1,
		})
		newAtIndex := len(stack)
		stack = appendTypeToStack(stack, t.Elem())

		// Link slice element to the slice frame.
		stack[newAtIndex].ParentFrameIndex = parentIndex

	case reflect.Struct:
		parentIndex := len(stack)
		numFields := t.NumField()
		stack = append(stack, stackFrame{
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
		stack = append(stack, stackFrame{
			Type:             ExpectTypeBool,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.String:
		stack = append(stack, stackFrame{
			Type:             ExpectTypeStr,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Int:
		stack = append(stack, stackFrame{
			Type:             ExpectTypeInt,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Int8:
		stack = append(stack, stackFrame{
			Type:             ExpectTypeInt8,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Int16:
		stack = append(stack, stackFrame{
			Type:             ExpectTypeInt16,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Int32:
		stack = append(stack, stackFrame{
			Type:             ExpectTypeInt32,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Int64:
		stack = append(stack, stackFrame{
			Type:             ExpectTypeInt64,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Uint:
		stack = append(stack, stackFrame{
			Type:             ExpectTypeUint,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Uint8:
		stack = append(stack, stackFrame{
			Type:             ExpectTypeUint8,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Uint16:
		stack = append(stack, stackFrame{
			Type:             ExpectTypeUint16,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Uint32:
		stack = append(stack, stackFrame{
			Type:             ExpectTypeUint32,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Uint64:
		stack = append(stack, stackFrame{
			Type:             ExpectTypeUint64,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Float32:
		stack = append(stack, stackFrame{
			Type:             ExpectTypeFloat32,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	case reflect.Float64:
		stack = append(stack, stackFrame{
			Type:             ExpectTypeFloat64,
			Size:             t.Size(),
			ParentFrameIndex: len(stack) - 1,
		})
	default:
		panic(fmt.Errorf("TODO: unsupported type: %v", t))
	}
	return stack
}

// Decode unmarshals the JSON contents of s into t.
// When S is string the decoder will not copy string values and will instead refer
// to the source string instead since Go strings are guaranteed to be immutable.
// When S is []byte all strings are copied.
func (d *Decoder[S, T]) Decode(s S, t *T) (err ErrorDecode) {
	defer func() {
		for i := range d.stackExp {
			d.stackExp[i].Dest = nil
		}
	}()
	// for i, v := range d.stackExp {
	// 	fmt.Printf("%d: %s (%d)\n", i, v.Type.String(), v.Size)
	// } // TODO: remove DEBUG

	if t == nil {
		return ErrorDecode{Err: ErrNilDest}
	}

	var fnA2I func(s S, dest unsafe.Pointer) error
	var fnA2UI func(s S, dest unsafe.Pointer) error

	if unsafe.Sizeof(int(0)) == 8 {
		// 64-bit system
		fnA2I = func(s S, dest unsafe.Pointer) error {
			i, overflow := atoi.I64(s)
			if overflow {
				return ErrIntegerOverflow
			}
			*(*int)(dest) = int(i)
			return nil
		}
		fnA2UI = func(s S, dest unsafe.Pointer) error {
			i, overflow := atoi.U64(s)
			if overflow {
				return ErrIntegerOverflow
			}
			*(*int)(dest) = int(i)
			return nil
		}
	} else {
		// 32-bit system
		fnA2I = func(s S, dest unsafe.Pointer) error {
			i, overflow := atoi.I32(s)
			if overflow {
				return ErrIntegerOverflow
			}
			*(*int)(dest) = int(i)
			return nil
		}
		fnA2UI = func(s S, dest unsafe.Pointer) error {
			i, overflow := atoi.U32(s)
			if overflow {
				return ErrIntegerOverflow
			}
			*(*int)(dest) = int(i)
			return nil
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
	// fmt.Println("ORIG PTR: ", d.stackExpect[0].Destination())

	errTok := d.tokenizer.Tokenize(s, func(tokens []jscan.Token) (exit bool) {
		// defer fmt.Println(" ")
		// fmt.Println("TOKENS: ", len(tokens), "/", cap(tokens))
		for ti := 0; ti < len(tokens); ti++ {
			// fmt.Println(" ")
			switch tokens[ti].Type {
			case jscan.TokenTypeFalse, jscan.TokenTypeTrue:
				if d.stackExp[si].Type != ExpectTypeBool {
					err = ErrorDecode{
						Err:   ErrUnexpectedValue,
						Index: tokens[ti].Index,
					}
					return true
				}
				p := unsafe.Pointer(uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset)
				*(*bool)(p) = tokens[ti].Type == jscan.TokenTypeTrue
				if d.stackExp[si].AdvanceBy > 0 {
					d.stackExp[si].Offset += d.stackExp[si].AdvanceBy
				}

			case jscan.TokenTypeInteger:
				p := unsafe.Pointer(uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset)
				switch d.stackExp[si].Type {
				case ExpectTypeUint:
					if s[tokens[ti].Index] == '-' {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
					if e := fnA2UI(s[tokens[ti].Index:tokens[ti].End], p); e != nil {
						// Invalid unsigned integer
						err = ErrorDecode{
							Err:   e,
							Index: tokens[ti].Index,
						}
						return true
					}

				case ExpectTypeInt:
					if e := fnA2I(s[tokens[ti].Index:tokens[ti].End], p); e != nil {
						// Invalid signed integer
						err = ErrorDecode{
							Err:   e,
							Index: tokens[ti].Index,
						}
						return true
					}

				case ExpectTypeUint8:
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
					if e := fnA2F32(s[tokens[ti].Index:tokens[ti].End], p); e != nil {
						err = ErrorDecode{Err: e, Index: tokens[ti].Index}
						return true
					}

				case ExpectTypeFloat64:
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
				if d.stackExp[si].AdvanceBy > 0 {
					d.stackExp[si].Offset += d.stackExp[si].AdvanceBy
				}

			case jscan.TokenTypeNumber:
				p := unsafe.Pointer(uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset)
				switch d.stackExp[si].Type {
				case ExpectTypeFloat32:
					if e := fnA2F32(s[tokens[ti].Index:tokens[ti].End], p); e != nil {
						err = ErrorDecode{Err: e, Index: tokens[ti].Index}
						return true
					}
				case ExpectTypeFloat64:
					if e := fnA2F64(s[tokens[ti].Index:tokens[ti].End], p); e != nil {
						err = ErrorDecode{Err: e, Index: tokens[ti].Index}
						return true
					}
				default:
					err = ErrorDecode{Err: ErrUnexpectedValue, Index: tokens[ti].Index}
					return true
				}
				if d.stackExp[si].AdvanceBy > 0 {
					d.stackExp[si].Offset += d.stackExp[si].AdvanceBy
				}

			case jscan.TokenTypeString:
				if d.stackExp[si].Type != ExpectTypeStr {
					err = ErrorDecode{
						Err:   ErrUnexpectedValue,
						Index: tokens[ti].Index,
					}
					return true
				}
				p := unsafe.Pointer(uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset)
				// This will either copy from a byte slice or create a substring
				*(*string)(p) = string(s[tokens[ti].Index+1 : tokens[ti].End-1])
				if d.stackExp[si].AdvanceBy > 0 {
					d.stackExp[si].Offset += d.stackExp[si].AdvanceBy
				}

			case jscan.TokenTypeNull:
				p := unsafe.Pointer(uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset)
				writeNullValue(d.stackExp[si].Type, p)
				if d.stackExp[si].AdvanceBy > 0 {
					d.stackExp[si].Offset += d.stackExp[si].AdvanceBy
				}

			case jscan.TokenTypeArray:
				switch d.stackExp[si].Type {
				case ExpectTypeArray:
					if tokens[ti].Elements < 1 {
						ti++ // Skip over closing TokenArrayEnd
						// Empty array, no need to allocate memory.
						// Go will automatically allocate it together with its parent
						// and zero it.
						break
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

					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					if d.stackExp[si].AdvanceBy > 0 {
						d.stackExp[si].Offset += d.stackExp[si].AdvanceBy
					}

					si++
					d.stackExp[si].Dest = p
					d.stackExp[si].Offset = 0
					d.stackExp[si].AdvanceBy = elementSize

				case ExpectTypeSlice:
					elementSize := d.stackExp[si+1].Size

					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					if d.stackExp[si].AdvanceBy > 0 {
						d.stackExp[si].Offset += d.stackExp[si].AdvanceBy
					}

					var dp unsafe.Pointer
					if elems := uintptr(tokens[ti].Elements); elems == 0 {
						// Allocate empty slice
						dp = allocate(elementSize)
						*(*sliceHeader)(p) = sliceHeader{Data: dp, Len: 0, Cap: 0}
					} else {
						dp = allocate(elems * elementSize)
						*(*sliceHeader)(p) = sliceHeader{Data: dp, Len: elems, Cap: elems}
					}
					si++
					d.stackExp[si].Dest = dp
					d.stackExp[si].Offset = 0
					d.stackExp[si].AdvanceBy = elementSize

				default:
					err = ErrorDecode{
						Err:   ErrUnexpectedValue,
						Index: tokens[ti].Index,
					}
					return true
				}

			case jscan.TokenTypeObject:
				switch d.stackExp[si].Type {
				// TODO: add support for map
				case ExpectTypeStruct:
					p := unsafe.Pointer(
						uintptr(d.stackExp[si].Dest) + d.stackExp[si].Offset,
					)
					if d.stackExp[si].AdvanceBy > 0 {
						d.stackExp[si].Offset += d.stackExp[si].AdvanceBy
					}
					// Point all fields to the struct, the offsets are already
					// set statically during decoder init time.
					for i := range d.stackExp[si].Fields {
						d.stackExp[d.stackExp[si].Fields[i].FrameIndex].Dest = p
					}
				default:
					err = ErrorDecode{
						Err:   ErrUnexpectedValue,
						Index: tokens[ti].Index,
					}
					return true
				}

			case jscan.TokenTypeKey:
				if tokens[ti-1].Type != jscan.TokenTypeObject {
					// Exit previous key first
					si = d.stackExp[si].ParentFrameIndex
					if si < 0 {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
				}
				switch d.stackExp[si].Type {
				case ExpectTypeStruct:
				SCAN_KEYVALS:
					for {
						key := s[tokens[ti].Index+1 : tokens[ti].End-1]
						frameIndex := fieldFrameIndexByName(d.stackExp[si].Fields, key)
						if frameIndex == -1 {
							// TODO: return error when option for unknown fields is enabled
							// Skip to the next key
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

				default:
					err = ErrorDecode{
						Err:   ErrUnexpectedValue,
						Index: tokens[ti].Index,
					}
					return true
				}
			case jscan.TokenTypeObjectEnd:
				// Go up the stack if current frame is a field
				if pi := d.stackExp[si].ParentFrameIndex; pi != -1 &&
					d.stackExp[pi].Fields != nil {
					si = d.stackExp[si].ParentFrameIndex
					if si < 0 {
						err = ErrorDecode{
							Err:   ErrUnexpectedValue,
							Index: tokens[ti].Index,
						}
						return true
					}
				}
			case jscan.TokenTypeArrayEnd:
				si = d.stackExp[si].ParentFrameIndex
				if si < 0 {
					err = ErrorDecode{
						Err:   ErrUnexpectedValue,
						Index: tokens[ti].Index,
					}
					return true
				}
			}
			// fmt.Printf("T%d: %s at %d\n", ti, tokens[ti].Type.String(), tokens[ti].Index)
			// fmt.Printf("expect: %#v\n", d.stackExpect[si].Type.String())
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

func writeNullValue(t ExpectType, p unsafe.Pointer) {
	switch t {
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
}

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
