package poly

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Shape is a sample interface for testing polymorphic JSON handling
type Shape interface {
}

// Circle is a concrete implementation of Shape
type Circle struct {
	Type   string  `json:"type"`
	Radius float64 `json:"radius"`
}

// Rect is another concrete implementation of Shape
type Rect struct {
	Type   string  `json:"type"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Request represents a sample request structure containing a Shape
type Request struct {
	Shape Shape `json:"shape"`
}

// RequestWithSlice represents a request with a slice of Shapes
type RequestWithSlice struct {
	Shapes []Shape `json:"shapes"`
}

// NestedRequest represents a request with nested structures
type NestedRequest struct {
	Data struct {
		Shape Shape `json:"shape"`
	} `json:"data"`
}

func TestMarshal(t *testing.T) {
	req := &Request{
		Shape: &Circle{Radius: 10},
	}
	var poly Poly
	require.NoError(t, poly.RegisterInterface((*Shape)(nil), "type"))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Circle)(nil), "circle"))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Rect)(nil), "rect"))
	require.NoError(t, poly.BeforeMarshalJSON(req, true))
	buf, err := json.Marshal(req)
	require.NoError(t, err)
	require.Equal(t, `{"shape":{"type":"circle","radius":10}}`, string(buf))
}

func TestUnmarshalJSON(t *testing.T) {
	req := &Request{}
	var poly Poly
	require.NoError(t, poly.RegisterInterface((*Shape)(nil), "type"))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Circle)(nil), "circle"))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Rect)(nil), "rect"))
	buf := []byte(`{"shape":{"type":"circle","radius":10}}`)
	require.NoError(t, poly.BeforeUnmarshalJSON(buf, req, true))
	require.NoError(t, json.Unmarshal(buf, &req))
	circle, ok := req.Shape.(*Circle)
	require.True(t, ok)
	require.Equal(t, &Circle{Radius: 10, Type: "circle"}, circle)
}

func TestRectUnmarshalJSON(t *testing.T) {
	req := &Request{}
	var poly Poly
	require.NoError(t, poly.RegisterInterface((*Shape)(nil), "type"))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Circle)(nil), "circle"))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Rect)(nil), "rect"))
	buf := []byte(`{"shape":{"type":"rect","width":5,"height":3}}`)
	require.NoError(t, poly.BeforeUnmarshalJSON(buf, req, true))
	require.NoError(t, json.Unmarshal(buf, &req))
	rect, ok := req.Shape.(*Rect)
	require.True(t, ok)
	require.Equal(t, &Rect{Width: 5, Height: 3, Type: "rect"}, rect)
}

func TestSliceMarshalUnmarshal(t *testing.T) {
	req := &RequestWithSlice{
		Shapes: []Shape{
			&Circle{Radius: 10},
			&Rect{Width: 5, Height: 3},
		},
	}
	var poly Poly
	require.NoError(t, poly.RegisterInterface((*Shape)(nil), "type"))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Circle)(nil), "circle"))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Rect)(nil), "rect"))

	// Marshal test
	require.NoError(t, poly.BeforeMarshalJSON(req, true))
	buf, err := json.Marshal(req)
	require.NoError(t, err)
	expected := `{"shapes":[{"type":"circle","radius":10},{"type":"rect","width":5,"height":3}]}`
	require.JSONEq(t, expected, string(buf))

	// Unmarshal test
	req2 := &RequestWithSlice{}
	require.NoError(t, poly.BeforeUnmarshalJSON(buf, req2, true))
	require.NoError(t, json.Unmarshal(buf, &req2))
	require.Len(t, req2.Shapes, 2)

	circle, ok := req2.Shapes[0].(*Circle)
	require.True(t, ok)
	require.Equal(t, &Circle{Radius: 10, Type: "circle"}, circle)

	rect, ok := req2.Shapes[1].(*Rect)
	require.True(t, ok)
	require.Equal(t, &Rect{Width: 5, Height: 3, Type: "rect"}, rect)
}

func TestNestedStructure(t *testing.T) {
	req := &NestedRequest{}
	req.Data.Shape = &Circle{Radius: 10}

	var poly Poly
	require.NoError(t, poly.RegisterInterface((*Shape)(nil), "type"))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Circle)(nil), "circle"))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Rect)(nil), "rect"))

	// Marshal test
	require.NoError(t, poly.BeforeMarshalJSON(req, true))
	buf, err := json.Marshal(req)
	require.NoError(t, err)
	expected := `{"data":{"shape":{"type":"circle","radius":10}}}`
	require.Equal(t, expected, string(buf))

	// Unmarshal test
	req2 := &NestedRequest{}
	require.NoError(t, poly.BeforeUnmarshalJSON(buf, req2, true))
	require.NoError(t, json.Unmarshal(buf, &req2))

	circle, ok := req2.Data.Shape.(*Circle)
	require.True(t, ok)
	require.Equal(t, &Circle{Radius: 10, Type: "circle"}, circle)
}

func TestRegisterErrors(t *testing.T) {
	var poly Poly

	// Test registering non-pointer interface
	// Note: Shape(nil) is actually a nil interface value, not a pointer to an interface
	// We need to pass a non-nil value that is not a pointer
	err := poly.RegisterInterface(struct{}{}, "type")
	require.Error(t, err)
	require.Contains(t, err.Error(), "poly: iFacePtr must be a pointer")

	// Test registering non-interface pointer
	err = poly.RegisterInterface((*string)(nil), "type")
	require.Error(t, err)
	require.Contains(t, err.Error(), "poly: iFacePtr must be a pointer to interface")

	// Test registering non-pointer struct
	err = poly.RegisterStruct((*Shape)(nil), Circle{}, "circle")
	require.Error(t, err)
	require.Contains(t, err.Error(), "poly: struct pointer must be a pointer")

	// Test registering non-struct pointer
	err = poly.RegisterStruct((*Shape)(nil), (*string)(nil), "circle")
	require.Error(t, err)
	require.Contains(t, err.Error(), "poly: struct pointer must be a pointer to a struct")

	// Test registering struct that doesn't implement interface
	type NotShape struct {
		Type string `json:"type"`
	}
	err = poly.RegisterStruct((*Shape)(nil), (*NotShape)(nil), "circle")
	require.Error(t, err)
	// Test registering struct without discriminant field
	type CircleWithoutType struct {
		Radius float64 `json:"radius"`
	}
	err = poly.RegisterStruct((*Shape)(nil), (*CircleWithoutType)(nil), "circle")
	require.Error(t, err)
	require.Contains(t, err.Error(), "poly: interface type")

	// Test duplicate interface registration
	require.NoError(t, poly.RegisterInterface((*Shape)(nil), "type"))
	err = poly.RegisterInterface((*Shape)(nil), "type")
	require.Error(t, err)
	require.Contains(t, err.Error(), "poly: interface already registered")
}

func TestBeforeMarshalErrors(t *testing.T) {
	var poly Poly
	require.NoError(t, poly.RegisterInterface((*Shape)(nil), "type"))

	// Test marshaling unregistered interface
	type AnotherShape interface{}
	req := &struct {
		Shape AnotherShape `json:"shape"`
	}{
		Shape: &Circle{Radius: 10},
	}
	err := poly.BeforeMarshalJSON(req, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "poly: interface type")
	require.Contains(t, err.Error(), "not registered")

	// Test marshaling unregistered struct
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Circle)(nil), "circle"))
	req2 := &Request{
		Shape: &Rect{Width: 5, Height: 3}, // Rect registered but not for this poly instance in this test
	}
	err = poly.BeforeMarshalJSON(req2, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "poly: interface type")
	require.Contains(t, err.Error(), "not found in struct")
}

func TestBeforeUnmarshalErrors(t *testing.T) {
	var poly Poly
	require.NoError(t, poly.RegisterInterface((*Shape)(nil), "type"))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Circle)(nil), "circle"))

	// Test unmarshaling unregistered interface
	type AnotherShape interface{}
	req := &struct {
		Shape AnotherShape `json:"shape"`
	}{}
	buf := []byte(`{"shape":{"type":"circle","radius":10}}`)
	err := poly.BeforeUnmarshalJSON(buf, req, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "poly: interface type")
	require.Contains(t, err.Error(), "not registered")

	// Test unmarshaling with unknown discriminant value
	req2 := &Request{}
	buf2 := []byte(`{"shape":{"type":"triangle","radius":10}}`)
	err = poly.BeforeUnmarshalJSON(buf2, req2, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "poly: cannot resolve interface")
	require.Contains(t, err.Error(), "type by field path")
}

func TestDefaultValue(t *testing.T) {
	var poly Poly
	require.NoError(t, poly.RegisterInterface((*Shape)(nil), "type"))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Circle)(nil), ""))

	buf := []byte(`{"shape":{"radius":10}}`)
	req := &struct {
		Shape Shape `json:"shape"`
	}{}
	err := poly.BeforeUnmarshalJSON(buf, req, true)
	require.NoError(t, err)
	_, ok := req.Shape.(*Circle)
	require.True(t, ok)
}
