// Package poly provides utilities for handling polymorphic JSON serialization and deserialization.
// It allows registering interfaces and their implementations, then automatically sets discriminant fields
// during marshaling and creates appropriate concrete types during unmarshaling.
package poly

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// polyType holds registration information for a specific interface type
type polyType struct {
	// fieldType is the reflect.Type of the interface
	fieldType reflect.Type

	// discriminantFieldName is the JSON field name used to distinguish implementations
	discriminantFieldName string

	// structValues contains the discriminant values for each registered struct
	structValues []any

	// structCreators are functions that create new instances of registered structs
	structCreators []func() any

	// structTypes are the reflect.Types of the registered structs
	structTypes []reflect.Type

	// structFieldPos tracks the position of the discriminant field in each struct
	structFieldPos []int
}

// Poly manages the registration of interfaces and their implementations for polymorphic JSON handling
type Poly struct {
	types map[string]*polyType
}

// RegisterInterface registers an interface type for polymorphic handling
// iFacePtr: a pointer to the interface type (e.g., (*Shape)(nil))
// discriminantFieldName: the JSON field name used to distinguish implementations (e.g., "type")
// discriminantFieldParser: a function to parse the discriminant field value from raw JSON
func (p *Poly) RegisterInterface(
	iFacePtr any,
	discriminantFieldName string) error {
	iFaceType, err := p.iFaceType(iFacePtr)
	if err != nil {
		return err
	}
	key := iFaceType.PkgPath() + "." + iFaceType.Name()
	if p.types == nil {
		p.types = make(map[string]*polyType)
	}
	_, ok := p.types[key]
	if ok {
		return errors.New("poly: interface already registered")
	}
	p.types[key] = &polyType{
		fieldType:             iFaceType,
		discriminantFieldName: discriminantFieldName,
	}
	return nil
}

// structType validates and extracts the reflect.Type from a struct pointer
func (p *Poly) structType(structPtr any) (reflect.Type, error) {
	structPtrType := reflect.TypeOf(structPtr)
	if structPtrType.Kind() != reflect.Ptr {
		return nil, errors.New("poly: struct pointer must be a pointer")
	}

	structType := structPtrType.Elem()
	if structType.Kind() != reflect.Struct {
		return nil, errors.New("poly: struct pointer must be a pointer to a struct")
	}
	return structType, nil
}

// iFaceType validates and extracts the reflect.Type from an interface pointer
func (p *Poly) iFaceType(iFacePtr any) (reflect.Type, error) {
	iFacePtrType := reflect.TypeOf(iFacePtr)
	if iFacePtrType.Kind() != reflect.Ptr {
		return nil, errors.New("poly: iFacePtr must be a pointer")
	}
	iFaceType := iFacePtrType.Elem()
	if iFaceType.Kind() != reflect.Interface {
		return nil, errors.New("poly: iFacePtr must be a pointer to interface")
	}
	return iFaceType, nil
}

// RegisterStruct registers a struct implementation for an interface
// iFacePtr: a pointer to the interface type (e.g., (*Shape)(nil))
// structPtr: a pointer to the struct type (e.g., (*Circle)(nil))
// value: the discriminant value for this struct (e.g., "circle")
func (p *Poly) RegisterStruct(
	iFacePtr any,
	structPtr any,
	value any) error {
	iFaceType, err := p.iFaceType(iFacePtr)
	if err != nil {
		return err
	}
	structType, err := p.structType(structPtr)
	if err != nil {
		return err
	}
	if !reflect.PointerTo(structType).Implements(iFaceType) {
		return errors.New("poly: interface type mismatch, struct ptr must implements interface")
	}
	key := iFaceType.PkgPath() + "." + iFaceType.Name()
	entry, ok := p.types[key]
	if !ok {
		return fmt.Errorf("poly: interface type %s not registered", key)
	}
	structFieldPos := -1
	for i := 0; i < structType.NumField(); i++ {
		f := structType.Field(i)
		jsonTag := f.Tag.Get("json")
		if jsonTag == "" {
			continue
		}
		fieldName := strings.Split(jsonTag, ",")[0]
		if fieldName == entry.discriminantFieldName {
			structFieldPos = i
			break
		}
	}
	if structFieldPos == -1 {
		return fmt.Errorf("poly: interface type %s not found in struct", key)
	}

	entry.structValues = append(entry.structValues, value)
	entry.structCreators = append(entry.structCreators, func() any {
		return reflect.New(structType).Interface()
	})
	entry.structTypes = append(entry.structTypes, structType)
	entry.structFieldPos = append(entry.structFieldPos, structFieldPos)

	return nil
}

// beforeMarshalJSONValue recursively processes values before JSON marshaling
// It sets discriminant field values for interface implementations
func (p *Poly) beforeMarshalJSONValue(val reflect.Value, strict bool) error {
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	// if it is interface
	if val.Kind() == reflect.Interface {
		iFaceType := val.Type()
		key := iFaceType.PkgPath() + "." + iFaceType.Name()
		entry, ok := p.types[key]
		if !ok { // is interface and not found
			if strict {
				return fmt.Errorf("poly: interface type %s not registered", key)
			} else {
				return nil
			}
		}
		val = val.Elem()
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		found := false
		for pos, sType := range entry.structTypes {
			if sType != val.Type() {
				continue
			}
			fieldOffset := entry.structFieldPos[pos]
			val.Field(fieldOffset).Set(reflect.ValueOf(entry.structValues[pos]))
			found = true
			break
		}
		if !found {
			return fmt.Errorf("poly: interface type %s not found in struct", key)
		}
	}
	if val.Kind() == reflect.Struct {
		for i := 0; i < val.NumField(); i++ {
			err := p.beforeMarshalJSONValue(val.Field(i), strict)
			if err != nil {
				return err
			}
		}
	} else if val.Kind() == reflect.Slice {
		for i := 0; i < val.Len(); i++ {
			err := p.beforeMarshalJSONValue(val.Index(i), strict)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// BeforeMarshalJSON prepares a value for JSON marshaling by setting discriminant fields
// Call this before json.Marshal to ensure interface implementations are correctly tagged
func (p *Poly) BeforeMarshalJSON(ptr any, strict bool) error {
	return p.beforeMarshalJSONValue(reflect.ValueOf(ptr), strict)
}

// beforeUnmarshalJSONValue recursively processes values before JSON unmarshaling
// It creates appropriate concrete types based on discriminant field values
func (p *Poly) beforeUnmarshalJSONValue(prefix []string, val reflect.Value, buf []byte, strict bool) error {
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() == reflect.Interface {
		iFaceType := val.Type()
		key := iFaceType.PkgPath() + "." + iFaceType.Name()
		entry, ok := p.types[key]
		if !ok {
			if strict {
				return fmt.Errorf("poly: interface type %s not registered", key)
			} else {
				return nil
			}
		}
		fieldName := entry.discriminantFieldName

		fieldPath := strings.Join(append(prefix, fieldName), ".")
		inputVal := gjson.GetBytes(buf, fieldPath)
		var iVal any
		if inputVal.Exists() {
			iVal = inputVal.Value()
		} else {
			iVal = reflect.New(reflect.TypeOf(entry.structValues[0])).Elem().Interface()
		}

		set := false
		for pos, dVal := range entry.structValues {
			if iVal != dVal {
				continue
			}
			refVal := reflect.ValueOf(entry.structCreators[pos]())
			val.Set(refVal)
			set = true
			break
		}
		if !set {
			return fmt.Errorf("poly: cannot resolve interface %s type by field path %s, raw json %s ", key, fieldPath, string(buf))
		}
		val = val.Elem()
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
	}
	if val.Kind() == reflect.Struct {
		for i := 0; i < val.NumField(); i++ {
			jsonTag := val.Type().Field(i).Tag.Get("json")
			if jsonTag == "" {
				continue
			}
			fieldName := strings.Split(jsonTag, ",")[0]
			if fieldName == "" {
				continue
			}
			err := p.beforeUnmarshalJSONValue(append(prefix, fieldName), val.Field(i), buf, strict)
			if err != nil {
				return err
			}
		}
	} else if val.Kind() == reflect.Slice {
		l := gjson.GetBytes(buf, strings.Join(append(prefix, "#"), ".")).Int()
		val.Set(reflect.MakeSlice(val.Type(), int(l), int(l)))

		for i := 0; i < val.Len(); i++ {
			err := p.beforeUnmarshalJSONValue(append(prefix, strconv.Itoa(i)), val.Index(i), buf, strict)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// BeforeUnmarshalJSON prepares a value for JSON unmarshaling by creating appropriate concrete types
// Call this before json.Unmarshal to ensure interface fields get the correct concrete implementations
// ptr: pointer to the value to populate
// buf: the JSON bytes to parse
func (p *Poly) BeforeUnmarshalJSON(buf []byte, ptr any, strict bool) error {
	return p.beforeUnmarshalJSONValue(nil, reflect.ValueOf(ptr), buf, strict)
}
