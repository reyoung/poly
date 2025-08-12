# Poly

Poly is a Go library for handling polymorphic JSON serialization and deserialization. It allows you to register interfaces and their implementations, then automatically sets discriminant fields during marshaling and creates appropriate concrete types during unmarshaling.

## Features

- Automatic discriminant field handling for JSON marshaling/unmarshaling
- Support for nested structures and slices
- Comprehensive error handling and validation
- Easy registration of interfaces and implementations

## Installation

```bash
go get github.com/reyoung/poly
```

## Usage

### Basic Example

```go
package main

import (
    "encoding/json"
    "fmt"
    "github.com/reyoung/poly"
)

// Define an interface
type Shape interface{}

// Implement the interface with concrete types
type Circle struct {
    Type   string  `json:"type"`
    Radius float64 `json:"radius"`
}

type Rect struct {
    Type   string  `json:"type"`
    Width  float64 `json:"width"`
    Height float64 `json:"height"`
}

// Define a struct that uses the interface
type Request struct {
    Shape Shape `json:"shape"`
}

func main() {
    // Create a poly instance
    var p poly.Poly
    
    // Register the interface
    p.RegisterInterface((*Shape)(nil), "type", func(message json.RawMessage) (any, error) {
        var str string
        if err := json.Unmarshal(message, &str); err != nil {
            return nil, err
        }
        return str, nil
    })
    
    // Register the implementations
    p.RegisterStruct((*Shape)(nil), (*Circle)(nil), "circle")
    p.RegisterStruct((*Shape)(nil), (*Rect)(nil), "rect")
    
    // Marshal example
    req := &Request{
        Shape: &Circle{Radius: 10},
    }
    p.BeforeMarshalJSON(req)
    buf, _ := json.Marshal(req)
    fmt.Println(string(buf)) // {"shape":{"type":"circle","radius":10}}
    
    // Unmarshal example
    req2 := &Request{}
    buf2 := []byte(`{"shape":{"type":"rect","width":5,"height":3}}`)
    p.BeforeUnmarshalJSON(req2, buf2)
    json.Unmarshal(buf2, &req2)
    rect, _ := req2.Shape.(*Rect)
    fmt.Printf("%+v\n", rect) // &{Type:rect Width:5 Height:3}
}
```

### Working with Slices

The library also supports slices of interface types:

```go
type RequestWithSlice struct {
    Shapes []Shape `json:"shapes"`
}

req := &RequestWithSlice{
    Shapes: []Shape{
        &Circle{Radius: 10},
        &Rect{Width: 5, Height: 3},
    },
}
```

## API Reference

### Poly
The main type for managing interface registrations.

#### Methods

- `RegisterInterface(iFacePtr any, discriminantFieldName string, discriminantFieldParser func(json.RawMessage) (any, error)) error`
  Registers an interface type for polymorphic handling.

- `RegisterStruct(iFacePtr any, structPtr any, value any) error`
  Registers a struct implementation for an interface.

- `BeforeMarshalJSON(ptr any) error`
  Prepares a value for JSON marshaling by setting discriminant fields.

- `BeforeUnmarshalJSON(ptr any, buf []byte) error`
  Prepares a value for JSON unmarshaling by creating appropriate concrete types.

## License

MIT