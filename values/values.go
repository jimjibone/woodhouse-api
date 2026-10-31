// Package values builds and reads the API's Value, FieldValue and Spec
// messages, converts schema fields to specs, and validates values against
// specs. It is shared by the server, the wh client library and tools.
package values

import (
	"time"

	clientsapi "github.com/jimjibone/woodhouse-api/go/v2/clients"
)

// Bool returns a bool Value.
func Bool(v bool) *clientsapi.Value {
	return &clientsapi.Value{Kind: &clientsapi.Value_Bool{Bool: v}}
}

// Int returns an int Value.
func Int(v int64) *clientsapi.Value {
	return &clientsapi.Value{Kind: &clientsapi.Value_Int{Int: v}}
}

// Float returns a float Value.
func Float(v float64) *clientsapi.Value {
	return &clientsapi.Value{Kind: &clientsapi.Value_Float{Float: v}}
}

// Text returns a text Value.
func Text(v string) *clientsapi.Value {
	return &clientsapi.Value{Kind: &clientsapi.Value_Text{Text: v}}
}

// Duration returns a duration Value (sent as milliseconds).
func Duration(v time.Duration) *clientsapi.Value {
	return &clientsapi.Value{Kind: &clientsapi.Value_Duration{Duration: v.Milliseconds()}}
}

// Time returns a time Value. The zero time is sent as 0 seconds, 0 nanos.
func Time(v time.Time) *clientsapi.Value {
	return &clientsapi.Value{Kind: &clientsapi.Value_Time{Time: TimeToPb(v)}}
}

// Color returns a color Value. Either encoding may be nil.
func Color(hueSat *clientsapi.ColorHueSat, xy *clientsapi.ColorXY) *clientsapi.Value {
	return &clientsapi.Value{Kind: &clientsapi.Value_Color{Color: &clientsapi.ColorValue{HueSat: hueSat, Xy: xy}}}
}

// HueSat returns a hue/saturation color Value.
func HueSat(hue, sat float64) *clientsapi.Value {
	return Color(&clientsapi.ColorHueSat{Hue: hue, Sat: sat}, nil)
}

// XY returns a CIE 1931 xy color Value.
func XY(x, y float64) *clientsapi.Value {
	return Color(nil, &clientsapi.ColorXY{X: x, Y: y})
}

// Enum returns an enum Value.
func Enum(v string) *clientsapi.Value {
	return &clientsapi.Value{Kind: &clientsapi.Value_Enum{Enum: v}}
}

// Image returns an image Value. Images carry no data; it marks the attribute
// as having an image to fetch.
func Image() *clientsapi.Value {
	return &clientsapi.Value{Kind: &clientsapi.Value_Image{Image: &clientsapi.ImageValue{}}}
}

// List returns a list Value.
func List(items ...*clientsapi.Value) *clientsapi.Value {
	return &clientsapi.Value{Kind: &clientsapi.Value_List{List: &clientsapi.ListValue{Items: items}}}
}

// Struct returns a struct Value.
func Struct(fields ...*clientsapi.FieldValue) *clientsapi.Value {
	return &clientsapi.Value{Kind: &clientsapi.Value_Struct{Struct: &clientsapi.StructValue{Fields: fields}}}
}

// Field returns a FieldValue.
func Field(id string, v *clientsapi.Value) *clientsapi.FieldValue {
	return &clientsapi.FieldValue{Id: id, Value: v}
}

// TimeToPb converts a time. The zero time becomes 0 seconds, 0 nanos.
func TimeToPb(t time.Time) *clientsapi.TimeValue {
	if t.IsZero() {
		return &clientsapi.TimeValue{}
	}
	return &clientsapi.TimeValue{Seconds: t.Unix(), Nanos: int32(t.Nanosecond())}
}

// TimeFromPb converts a time. nil and 0 seconds, 0 nanos become the zero time.
func TimeFromPb(t *clientsapi.TimeValue) time.Time {
	if t == nil || (t.Seconds == 0 && t.Nanos == 0) {
		return time.Time{}
	}
	return time.Unix(t.Seconds, int64(t.Nanos))
}

// IsSet reports whether a Value holds any kind.
func IsSet(v *clientsapi.Value) bool {
	return v != nil && v.Kind != nil
}

// Kind returns the name of a Value's kind ("bool", "int", ...), or "" if
// unset. The names match schema.Type.
func Kind(v *clientsapi.Value) string {
	switch v.GetKind().(type) {
	case *clientsapi.Value_Bool:
		return "bool"
	case *clientsapi.Value_Int:
		return "int"
	case *clientsapi.Value_Float:
		return "float"
	case *clientsapi.Value_Text:
		return "text"
	case *clientsapi.Value_Duration:
		return "duration"
	case *clientsapi.Value_Time:
		return "time"
	case *clientsapi.Value_Color:
		return "color"
	case *clientsapi.Value_Enum:
		return "enum"
	case *clientsapi.Value_Image:
		return "image"
	case *clientsapi.Value_List:
		return "list"
	case *clientsapi.Value_Struct:
		return "struct"
	}
	return ""
}

// AsDuration reads a duration Value.
func AsDuration(v *clientsapi.Value) (time.Duration, bool) {
	d, ok := v.GetKind().(*clientsapi.Value_Duration)
	if !ok {
		return 0, false
	}
	return time.Duration(d.Duration) * time.Millisecond, true
}

// AsTime reads a time Value.
func AsTime(v *clientsapi.Value) (time.Time, bool) {
	t, ok := v.GetKind().(*clientsapi.Value_Time)
	if !ok {
		return time.Time{}, false
	}
	return TimeFromPb(t.Time), true
}

// Fields indexes field values by ID. Later duplicates win.
func Fields(fields []*clientsapi.FieldValue) map[string]*clientsapi.Value {
	m := make(map[string]*clientsapi.Value, len(fields))
	for _, f := range fields {
		m[f.GetId()] = f.GetValue()
	}
	return m
}
