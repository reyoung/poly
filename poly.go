package poly

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

type polyType struct {
	fieldType               reflect.Type
	discriminantFieldName   string
	discriminantFieldParser func(json.RawMessage) (interface{}, error)
	structValues            []any
	structCreators          []func() any
	structTypes             []reflect.Type
	structFieldPos          []int
}

type Poly struct {
	types map[string]*polyType
}

func (p *Poly) RegisterInterface(
	iFacePtr any,
	discriminantFieldName string,
	discriminantFieldParser func(json.RawMessage) (any, error)) error {
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
		fieldType:               iFaceType,
		discriminantFieldName:   discriminantFieldName,
		discriminantFieldParser: discriminantFieldParser,
	}
	return nil
}

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

func (p *Poly) beforeMarshalJSONValue(val reflect.Value) error {
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	// if it is interface
	if val.Kind() == reflect.Interface {
		iFaceType := val.Type()
		key := iFaceType.PkgPath() + "." + iFaceType.Name()
		entry, ok := p.types[key]
		if !ok {
			return fmt.Errorf("poly: interface type %s not registered", key)
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
			err := p.beforeMarshalJSONValue(val.Field(i))
			if err != nil {
				return err
			}
		}
	} else if val.Kind() == reflect.Slice {
		for i := 0; i < val.Len(); i++ {
			err := p.beforeMarshalJSONValue(val.Index(i))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Poly) BeforeMarshalJSON(ptr any) error {
	return p.beforeMarshalJSONValue(reflect.ValueOf(ptr))
}
func (p *Poly) beforeUnmarshalJSONValue(prefix []string, val reflect.Value, buf []byte) error {
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() == reflect.Interface {
		iFaceType := val.Type()
		key := iFaceType.PkgPath() + "." + iFaceType.Name()
		entry, ok := p.types[key]
		if !ok {
			return fmt.Errorf("poly: interface type %s not registered", key)
		}
		fieldName := entry.discriminantFieldName

		fieldPath := strings.Join(append(prefix, fieldName), ".")
		iVal := gjson.GetBytes(buf, fieldPath).Value()

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
			return fmt.Errorf("poly: interface type %s not found in struct", key)
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
			err := p.beforeUnmarshalJSONValue(append(prefix, fieldName), val.Field(i), buf)
			if err != nil {
				return err
			}
		}
	} else if val.Kind() == reflect.Slice {
		for i := 0; i < val.Len(); i++ {
			err := p.beforeUnmarshalJSONValue(append(prefix, "["+strconv.Itoa(i)+"]"), val.Index(i), buf)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Poly) BeforeUnmarshalJSON(ptr any, buf []byte) error {
	return p.beforeUnmarshalJSONValue(nil, reflect.ValueOf(ptr), buf)
}
