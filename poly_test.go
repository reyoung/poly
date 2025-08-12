package poly

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type Shape interface {
}

type Circle struct {
	Type   string  `json:"type"`
	Radius float64 `json:"radius"`
}

type Rect struct {
	Type   string  `json:"type"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type Request struct {
	Shape Shape `json:"shape"`
}

func TestMarshal(t *testing.T) {
	req := &Request{
		Shape: &Circle{Radius: 10},
	}
	var poly Poly
	require.NoError(t, poly.RegisterInterface((*Shape)(nil), "type", func(message json.RawMessage) (any, error) {
		var str string
		if err := json.Unmarshal(message, &str); err != nil {
			return nil, err
		}
		return str, nil
	}))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Circle)(nil), "circle"))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Rect)(nil), "rect"))
	require.NoError(t, poly.BeforeMarshalJSON(req))
	buf, err := json.Marshal(req)
	require.NoError(t, err)
	require.Equal(t, `{"shape":{"type":"circle","radius":10}}`, string(buf))
}

func TestUnmarshalJSON(t *testing.T) {
	req := &Request{}
	var poly Poly
	require.NoError(t, poly.RegisterInterface((*Shape)(nil), "type", func(message json.RawMessage) (any, error) {
		var str string
		if err := json.Unmarshal(message, &str); err != nil {
			return nil, err
		}
		return str, nil
	}))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Circle)(nil), "circle"))
	require.NoError(t, poly.RegisterStruct((*Shape)(nil), (*Rect)(nil), "rect"))
	buf := []byte(`{"shape":{"type":"circle","radius":10}}`)
	require.NoError(t, poly.BeforeUnmarshalJSON(req, buf))
	require.NoError(t, json.Unmarshal(buf, &req))
	circle, ok := req.Shape.(*Circle)
	require.True(t, ok)
	require.Equal(t, &Circle{Radius: 10, Type: "circle"}, circle)
}
