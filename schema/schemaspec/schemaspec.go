// Package schemaspec converts the service schema's fields, commands and events
// into API Specs, for the enabled features of a service. It is used at
// generate time (wh's whgen) and by tests; runtime code works from the specs
// the generated code or the wire carries, not from the schema.
package schemaspec

import (
	"fmt"
	"slices"

	clientsapi "github.com/jimjibone/woodhouse-api/go/v2/clients"
	"github.com/jimjibone/woodhouse-api/schema"
)

// Enabled reports whether a schema field, command or event that names
// fieldFeatures is present given the enabled features: it is when it names
// none, or any of them is enabled.
func Enabled(fieldFeatures []string, enabled []string) bool {
	if len(fieldFeatures) == 0 {
		return true
	}
	for _, f := range fieldFeatures {
		if slices.Contains(enabled, f) {
			return true
		}
	}
	return false
}

// Unit converts a schema unit name ("celsius") to the proto Unit. Unknown or
// empty names give UNIT_UNDEFINED.
func Unit(name string) clientsapi.Unit {
	if name == "" {
		return clientsapi.Unit_UNIT_UNDEFINED
	}
	return clientsapi.Unit(clientsapi.Unit_value[schema.UnitProtoName(name)])
}

// Perms converts schema perms to the proto Permissions.
func Perms(p schema.Perms) clientsapi.Permissions {
	if p == schema.PermsRW {
		return clientsapi.Permissions_PERM_READWRITE
	}
	return clientsapi.Permissions_PERM_READONLY
}

// SpecFromSchema converts a schema field's type and constraints to a Spec.
// A color field accepts both encodings; providers narrow it if needed.
func SpecFromSchema(f *schema.Field) *clientsapi.Spec {
	switch f.Type {
	case schema.TypeBool:
		return &clientsapi.Spec{Kind: &clientsapi.Spec_Bool{Bool: &clientsapi.BoolSpec{}}}
	case schema.TypeInt:
		s := &clientsapi.IntSpec{Unit: Unit(f.Unit)}
		if f.Min != nil {
			v := int64(*f.Min)
			s.Min = &v
		}
		if f.Max != nil {
			v := int64(*f.Max)
			s.Max = &v
		}
		if f.Step != nil {
			v := uint64(*f.Step)
			s.Step = &v
		}
		return &clientsapi.Spec{Kind: &clientsapi.Spec_Int{Int: s}}
	case schema.TypeFloat:
		s := &clientsapi.FloatSpec{Unit: Unit(f.Unit), Min: f.Min, Max: f.Max, Step: f.Step}
		return &clientsapi.Spec{Kind: &clientsapi.Spec_Float{Float: s}}
	case schema.TypeText:
		return &clientsapi.Spec{Kind: &clientsapi.Spec_Text{Text: &clientsapi.TextSpec{}}}
	case schema.TypeDuration:
		s := &clientsapi.DurationSpec{}
		if f.MinDuration != nil {
			v := f.MinDuration.Milliseconds()
			s.Min = &v
		}
		if f.MaxDuration != nil {
			v := f.MaxDuration.Milliseconds()
			s.Max = &v
		}
		if f.StepDuration != nil {
			v := uint64(f.StepDuration.Milliseconds())
			s.Step = &v
		}
		return &clientsapi.Spec{Kind: &clientsapi.Spec_Duration{Duration: s}}
	case schema.TypeTime:
		return &clientsapi.Spec{Kind: &clientsapi.Spec_Time{Time: &clientsapi.TimeSpec{}}}
	case schema.TypeColor:
		return &clientsapi.Spec{Kind: &clientsapi.Spec_Color{Color: &clientsapi.ColorSpec{HueSat: true, Xy: true}}}
	case schema.TypeEnum:
		return &clientsapi.Spec{Kind: &clientsapi.Spec_Enum{Enum: &clientsapi.EnumSpec{Options: slices.Clone(f.Options)}}}
	case schema.TypeImage:
		return &clientsapi.Spec{Kind: &clientsapi.Spec_Image{Image: &clientsapi.ImageSpec{}}}
	case schema.TypeList:
		s := &clientsapi.ListSpec{Item: SpecFromSchema(f.Item)}
		if f.MinItems != nil {
			v := uint32(*f.MinItems)
			s.MinItems = &v
		}
		if f.MaxItems != nil {
			v := uint32(*f.MaxItems)
			s.MaxItems = &v
		}
		return &clientsapi.Spec{Kind: &clientsapi.Spec_List{List: s}}
	case schema.TypeStruct:
		s := &clientsapi.StructSpec{}
		for _, sub := range f.Fields {
			s.Fields = append(s.Fields, FieldSpecFromSchema(sub))
		}
		return &clientsapi.Spec{Kind: &clientsapi.Spec_Struct{Struct: s}}
	}
	panic(fmt.Sprintf("values: unknown schema type %q", f.Type))
}

// SpecForFeatures is SpecFromSchema for a field of a service with the given
// features enabled. A color field tied to features accepts hue_sat only when
// the feature "hs" is enabled and xy only when "xy" is (the schema's naming
// convention for color encodings).
func SpecForFeatures(f *schema.Field, features []string) *clientsapi.Spec {
	spec := SpecFromSchema(f)
	if c := spec.GetColor(); c != nil && len(f.Features) > 0 {
		c.HueSat = slices.Contains(features, "hs")
		c.Xy = slices.Contains(features, "xy")
	}
	return spec
}

// FieldSpecFromSchema converts a schema field to a FieldSpec.
func FieldSpecFromSchema(f *schema.Field) *clientsapi.FieldSpec {
	return &clientsapi.FieldSpec{Id: f.Name, Spec: SpecFromSchema(f), Required: f.Required}
}

// FieldSpecsFromSchema converts the schema fields enabled by the features.
func FieldSpecsFromSchema(fields []*schema.Field, features []string) []*clientsapi.FieldSpec {
	var specs []*clientsapi.FieldSpec
	for _, f := range fields {
		if Enabled(f.Features, features) {
			specs = append(specs, &clientsapi.FieldSpec{Id: f.Name, Spec: SpecForFeatures(f, features), Required: f.Required})
		}
	}
	return specs
}

// CommandSpecFromSchema converts a schema command, keeping the params and
// result fields enabled by the features.
func CommandSpecFromSchema(c *schema.Command, features []string) *clientsapi.CommandSpec {
	return &clientsapi.CommandSpec{
		Id:     c.Name,
		Params: FieldSpecsFromSchema(c.Params, features),
		Result: FieldSpecsFromSchema(c.Result, features),
	}
}

// EventSpecFromSchema converts a schema event, keeping the fields enabled by
// the features.
func EventSpecFromSchema(e *schema.Event, features []string) *clientsapi.EventSpec {
	return &clientsapi.EventSpec{Id: e.Name, Fields: FieldSpecsFromSchema(e.Fields, features)}
}
