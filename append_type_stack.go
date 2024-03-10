package jscandec

import (
	"fmt"
	"reflect"
	"strings"
	"unsafe"
)

// appendTypeToStack will recursively flat-append stack frames recursing into t.
func appendTypeToStack[S []byte | string](
	stack []stackFrame[S], t reflect.Type, options *InitOptions,
) ([]stackFrame[S], error) {
	if t == nil {
		return append(stack, stackFrame[S]{
			Type:             ExpectTypeAny,
			Typ:              getTyp(t),
			Size:             unsafe.Sizeof(struct{ typ, dat uintptr }{}),
			ParentFrameIndex: noParentFrame,
		}), nil
	} else if s := determineJSONUnmarshalerSupport(t); s != interfaceSupportNone {
		return append(stack, stackFrame[S]{
			Type:             ExpectTypeJSONUnmarshaler,
			Typ:              getTyp(t),
			RType:            t,
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		}), nil
	} else if s := determineTextUnmarshalerSupport(t); s != interfaceSupportNone {
		return append(stack, stackFrame[S]{
			Type:             ExpectTypeTextUnmarshaler,
			Typ:              getTyp(t),
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
			Typ:              getTyp(t),
			Size:             unsafe.Sizeof(struct{ typ, dat uintptr }{}),
			ParentFrameIndex: noParentFrame,
		}), nil
	case reflect.Array:
		if t.Len() == 0 {
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeArrayLen0,
				Typ:              getTyp(t),
				Size:             0,
				ParentFrameIndex: noParentFrame,
			}), nil
		}

		parentIndex := uint32(len(stack))
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeArray,
			Typ:              getTyp(t),
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
		newAtIndex := len(stack)
		var err error
		if stack, err = appendTypeToStack(stack, t.Elem(), options); err != nil {
			return nil, err
		}

		// Link array element to the array frame.
		stack[newAtIndex].ParentFrameIndex = parentIndex
		stack[newAtIndex].Cap = t.Len()

	case reflect.Slice:
		elem := t.Elem()
		switch elem.Kind() {
		case reflect.Struct:
			if elem.Size() < 1 {
				return append(stack, stackFrame[S]{
					Type:             ExpectTypeSliceEmptyStruct,
					Typ:              getTyp(t),
					Size:             t.Size(),
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
						Typ:              getTyp(t),
						Size:             t.Size(),
						ParentFrameIndex: noParentFrame,
						RecurFrame:       i,
					}), nil
				}
			}
		case reflect.Bool:
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeSliceBool,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.String:
			if elem == tpNumber {
				break
			}
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeSliceString,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Int:
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeSliceInt,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Int8:
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeSliceInt8,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Int16:
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeSliceInt16,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Int32:
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeSliceInt32,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Int64:
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeSliceInt64,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Uint:
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeSliceUint,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Uint8:
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeSliceUint8,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Uint16:
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeSliceUint16,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Uint32:
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeSliceUint32,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Uint64:
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeSliceUint64,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Float32:
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeSliceFloat32,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		case reflect.Float64:
			return append(stack, stackFrame[S]{
				Type:             ExpectTypeSliceFloat64,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		}

		parentIndex := uint32(len(stack))
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeSlice,
			Typ:              getTyp(t),
			Size:             t.Size(),
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
						Typ:                    getTyp(t),
						Size:                   t.Size(),
						MapValueType:           getTyp(t.Elem()),
						MapCanUseAssignFaststr: canUseAssignFaststr(t),
						ParentFrameIndex:       noParentFrame,
						RecurFrame:             i,
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
				Typ:              getTyp(t),
				RType:            t,
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		}

		stack = append(stack, stackFrame[S]{
			Type:                   ExpectTypeMap,
			Typ:                    getTyp(t),
			RType:                  t,
			Size:                   t.Size(),
			MapValueType:           getTyp(t.Elem()),
			MapCanUseAssignFaststr: canUseAssignFaststr(t),
			ParentFrameIndex:       noParentFrame,
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
				Type:             ExpectTypeEmptyStruct,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			}), nil
		}
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeStruct,
			Typ:              getTyp(t),
			RType:            t,
			Size:             t.Size(),
			Fields:           make([]fieldStackFrame, 0, numFields),
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
			Typ:              getTyp(t),
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.String:
		if t == tpNumber {
			stack = append(stack, stackFrame[S]{
				Type:             ExpectTypeNumber,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			})
		} else {
			stack = append(stack, stackFrame[S]{
				Type:             ExpectTypeStr,
				Typ:              getTyp(t),
				Size:             t.Size(),
				ParentFrameIndex: noParentFrame,
			})
		}
	case reflect.Int:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeInt,
			Typ:              getTyp(t),
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Int8:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeInt8,
			Typ:              getTyp(t),
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Int16:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeInt16,
			Typ:              getTyp(t),
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Int32:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeInt32,
			Typ:              getTyp(t),
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Int64:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeInt64,
			Typ:              getTyp(t),
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Uint:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeUint,
			Typ:              getTyp(t),
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Uint8:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeUint8,
			Typ:              getTyp(t),
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Uint16:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeUint16,
			Typ:              getTyp(t),
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Uint32:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeUint32,
			Typ:              getTyp(t),
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Uint64:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeUint64,
			Typ:              getTyp(t),
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Float32:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeFloat32,
			Typ:              getTyp(t),
			Size:             t.Size(),
			ParentFrameIndex: noParentFrame,
		})
	case reflect.Float64:
		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypeFloat64,
			Typ:              getTyp(t),
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
						Typ:              getTyp(t),
						Size:             t.Size(),
						ParentFrameIndex: noParentFrame,
						RecurFrame:       i,
					}), nil
				}
			}
		}

		stack = append(stack, stackFrame[S]{
			Type:             ExpectTypePtr,
			Typ:              getTyp(t),
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

func getTyp(t reflect.Type) *typ {
	return (*typ)(((*emptyInterface)(unsafe.Pointer(&t))).ptr)
}
