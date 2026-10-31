// Package schema loads and validates the Woodhouse service schema
// (services.yaml): the service types, their attributes, commands, events and
// feature flags. Code generators and tests use it as the single source of
// truth.
package schema

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

//go:embed services.yaml
var embedded []byte

// HeaderName is the gRPC metadata key every client sends with the schema
// version it was built against. The server rejects a mismatch.
const HeaderName = "woodhouse-schema-version"

// Type is the value type of a field.
type Type string

const (
	TypeBool     Type = "bool"
	TypeInt      Type = "int"
	TypeFloat    Type = "float"
	TypeText     Type = "text"
	TypeDuration Type = "duration"
	TypeTime     Type = "time"
	TypeColor    Type = "color"
	TypeEnum     Type = "enum"
	TypeImage    Type = "image"
	TypeList     Type = "list"
	TypeStruct   Type = "struct"
)

var types = map[Type]bool{
	TypeBool: true, TypeInt: true, TypeFloat: true, TypeText: true, TypeDuration: true, TypeTime: true,
	TypeColor: true, TypeEnum: true, TypeImage: true, TypeList: true, TypeStruct: true,
}

// Perms are the permissions of an attribute.
type Perms string

const (
	PermsRO Perms = "ro"
	PermsRW Perms = "rw"
)

// Units lists the unit names a field may use. Each maps to the proto Unit enum
// value "UNIT_" + upper case name.
var Units = []string{
	"percentage", "arc_degrees", "celsius", "lux", "seconds", "ppm", "micrograms_per_cubic_meter",
	"volts", "amps", "watts", "kelvin", "hectopascal", "meters", "kilowatt_hours",
}

// Schema is the parsed service schema.
type Schema struct {
	Version  int        `yaml:"version"`
	Services []*Service `yaml:"services"`
}

// Service is a service type.
type Service struct {
	Name       string     `yaml:"name"`
	Desc       string     `yaml:"desc"`
	Open       bool       `yaml:"open"`
	Features   []*Feature `yaml:"features"`
	Attributes []*Field   `yaml:"attributes"`
	Commands   []*Command `yaml:"commands"`
	Events     []*Event   `yaml:"events"`
}

// Feature is an optional capability of a service.
type Feature struct {
	Name string `yaml:"name"`
	Desc string `yaml:"desc"`
}

// Command is a command a service accepts.
type Command struct {
	Name     string   `yaml:"name"`
	Desc     string   `yaml:"desc"`
	Features []string `yaml:"features"`
	Params   []*Field `yaml:"params"`
	Result   []*Field `yaml:"result"`
}

// Event is an event a service emits.
type Event struct {
	Name     string   `yaml:"name"`
	Desc     string   `yaml:"desc"`
	Features []string `yaml:"features"`
	Fields   []*Field `yaml:"fields"`
}

// Field is an attribute, command parameter or result, event field, struct
// field or list item.
type Field struct {
	Name     string   `yaml:"name"`
	Desc     string   `yaml:"desc"`
	Type     Type     `yaml:"type"`
	Perms    Perms    `yaml:"perms"`
	Required bool     `yaml:"required"`
	Features []string `yaml:"features"`
	Unit     string   `yaml:"unit"`
	Options  []string `yaml:"options"`
	Item     *Field   `yaml:"item"`
	Fields   []*Field `yaml:"fields"`
	MinItems *int     `yaml:"min_items"`
	MaxItems *int     `yaml:"max_items"`

	// Constraints as written. Parsed into Min/Max/Step (int, float) or
	// MinDuration/MaxDuration/StepDuration (duration) by validation.
	RawMin  any `yaml:"min"`
	RawMax  any `yaml:"max"`
	RawStep any `yaml:"step"`

	Min, Max, Step                         *float64       `yaml:"-"`
	MinDuration, MaxDuration, StepDuration *time.Duration `yaml:"-"`
}

// Load parses and validates the embedded schema.
func Load() (*Schema, error) {
	return Parse(embedded)
}

// MustLoad is Load, panicking on error. The embedded schema is checked by the
// package tests, so this only fails on a broken build.
func MustLoad() *Schema {
	s, err := Load()
	if err != nil {
		panic(err)
	}
	return s
}

// Version returns the embedded schema's version, the value clients send in
// the HeaderName metadata.
func Version() int {
	return MustLoad().Version
}

// Raw returns the embedded schema file.
func Raw() []byte {
	return bytes.Clone(embedded)
}

// Parse parses and validates a schema. Unknown keys are errors.
func Parse(data []byte) (*Schema, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var s Schema
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

// Service returns the named service type, or nil.
func (s *Schema) Service(name string) *Service {
	for _, srv := range s.Services {
		if srv.Name == name {
			return srv
		}
	}
	return nil
}

// ProtoName returns the Service.ServiceType enum value name, e.g. "ON_OFF".
func (srv *Service) ProtoName() string {
	return strings.ToUpper(srv.Name)
}

// Feature returns the named feature, or nil.
func (srv *Service) Feature(name string) *Feature {
	for _, f := range srv.Features {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// Attribute returns the named attribute, or nil.
func (srv *Service) Attribute(name string) *Field {
	for _, a := range srv.Attributes {
		if a.Name == name {
			return a
		}
	}
	return nil
}

// Command returns the named command, or nil.
func (srv *Service) Command(name string) *Command {
	for _, c := range srv.Commands {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// Event returns the named event, or nil.
func (srv *Service) Event(name string) *Event {
	for _, e := range srv.Events {
		if e.Name == name {
			return e
		}
	}
	return nil
}

// UnitProtoName returns the proto Unit enum value name for a unit, e.g.
// "UNIT_CELSIUS".
func UnitProtoName(unit string) string {
	return "UNIT_" + strings.ToUpper(unit)
}

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func (s *Schema) validate() error {
	var errs []error
	fail := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	if s.Version <= 0 {
		fail("version must be positive, got %d", s.Version)
	}
	if len(s.Services) == 0 {
		fail("no services")
	}

	seen := map[string]bool{}
	for i, srv := range s.Services {
		where := fmt.Sprintf("services[%d] %q", i, srv.Name)
		if !nameRe.MatchString(srv.Name) {
			fail("%s: invalid name", where)
		}
		if seen[srv.Name] {
			fail("%s: duplicate service", where)
		}
		seen[srv.Name] = true
		if srv.Desc == "" {
			fail("%s: missing desc", where)
		}
		srv.validate(where, fail)
	}
	return errors.Join(errs...)
}

func (srv *Service) validate(where string, fail func(string, ...any)) {
	features := map[string]bool{}
	used := map[string]bool{}
	for _, f := range srv.Features {
		if !nameRe.MatchString(f.Name) {
			fail("%s: feature %q: invalid name", where, f.Name)
		}
		if features[f.Name] {
			fail("%s: feature %q: duplicate", where, f.Name)
		}
		features[f.Name] = true
	}
	useFeatures := func(at string, names []string) {
		for _, name := range names {
			if !features[name] {
				fail("%s: unknown feature %q", at, name)
			}
			used[name] = true
		}
	}

	names := map[string]bool{}
	for _, a := range srv.Attributes {
		at := fmt.Sprintf("%s: attribute %q", where, a.Name)
		if names[a.Name] {
			fail("%s: duplicate", at)
		}
		names[a.Name] = true
		if a.Perms == "" {
			a.Perms = PermsRO
		}
		if a.Perms != PermsRO && a.Perms != PermsRW {
			fail("%s: invalid perms %q", at, a.Perms)
		}
		useFeatures(at, a.Features)
		a.validate(at, true, fail)
	}

	names = map[string]bool{}
	for _, c := range srv.Commands {
		at := fmt.Sprintf("%s: command %q", where, c.Name)
		if !nameRe.MatchString(c.Name) {
			fail("%s: invalid name", at)
		}
		if names[c.Name] {
			fail("%s: duplicate", at)
		}
		names[c.Name] = true
		useFeatures(at, c.Features)
		validateFields(at+" params", c.Params, useFeatures, fail)
		validateFields(at+" result", c.Result, useFeatures, fail)
	}

	names = map[string]bool{}
	for _, e := range srv.Events {
		at := fmt.Sprintf("%s: event %q", where, e.Name)
		if !nameRe.MatchString(e.Name) {
			fail("%s: invalid name", at)
		}
		if names[e.Name] {
			fail("%s: duplicate", at)
		}
		names[e.Name] = true
		useFeatures(at, e.Features)
		validateFields(at+" fields", e.Fields, useFeatures, fail)
	}

	for name := range features {
		if !used[name] {
			fail("%s: feature %q is not used by any attribute, command, event or parameter", where, name)
		}
	}
}

// validateFields checks the top-level fields of a command or event. Their
// perms are meaningless, and they may name features.
func validateFields(where string, fields []*Field, useFeatures func(string, []string), fail func(string, ...any)) {
	names := map[string]bool{}
	for _, f := range fields {
		at := fmt.Sprintf("%s %q", where, f.Name)
		if names[f.Name] {
			fail("%s: duplicate", at)
		}
		names[f.Name] = true
		if f.Perms != "" {
			fail("%s: perms only apply to attributes", at)
		}
		useFeatures(at, f.Features)
		f.validate(at, true, fail)
	}
}

// validate checks a field and its nested item or fields. top is false for
// list items and struct fields, which cannot name features or perms.
func (f *Field) validate(at string, top bool, fail func(string, ...any)) {
	if f.Name != "" && !nameRe.MatchString(f.Name) {
		fail("%s: invalid name", at)
	}
	if !types[f.Type] {
		fail("%s: invalid type %q", at, f.Type)
		return
	}
	if !top {
		if len(f.Features) > 0 {
			fail("%s: features only apply to top-level fields", at)
		}
		if f.Perms != "" {
			fail("%s: perms only apply to attributes", at)
		}
	}
	if f.Required && len(f.Features) > 0 {
		fail("%s: required and features are mutually exclusive", at)
	}

	switch f.Type {
	case TypeInt, TypeFloat:
		f.Min = number(at, "min", f.RawMin, f.Type, fail)
		f.Max = number(at, "max", f.RawMax, f.Type, fail)
		f.Step = number(at, "step", f.RawStep, f.Type, fail)
		if f.Min != nil && f.Max != nil && *f.Min > *f.Max {
			fail("%s: min is greater than max", at)
		}
		if f.Step != nil && *f.Step <= 0 {
			fail("%s: step must be positive", at)
		}
	case TypeDuration:
		f.MinDuration = duration(at, "min", f.RawMin, fail)
		f.MaxDuration = duration(at, "max", f.RawMax, fail)
		f.StepDuration = duration(at, "step", f.RawStep, fail)
		if f.MinDuration != nil && f.MaxDuration != nil && *f.MinDuration > *f.MaxDuration {
			fail("%s: min is greater than max", at)
		}
		if f.StepDuration != nil && *f.StepDuration <= 0 {
			fail("%s: step must be positive", at)
		}
	default:
		if f.RawMin != nil || f.RawMax != nil || f.RawStep != nil {
			fail("%s: min, max and step only apply to int, float and duration", at)
		}
	}

	if f.Unit != "" {
		if f.Type != TypeInt && f.Type != TypeFloat {
			fail("%s: unit only applies to int and float", at)
		} else if !knownUnit(f.Unit) {
			fail("%s: unknown unit %q", at, f.Unit)
		}
	}

	if len(f.Options) > 0 {
		if f.Type != TypeEnum {
			fail("%s: options only apply to enum", at)
		}
		opts := map[string]bool{}
		for _, o := range f.Options {
			if o == "" || opts[o] {
				fail("%s: empty or duplicate option %q", at, o)
			}
			opts[o] = true
		}
	}

	if f.Type == TypeList {
		if f.Item == nil {
			fail("%s: list needs an item", at)
		} else {
			f.Item.validate(at+" item", false, fail)
		}
		if f.MinItems != nil && *f.MinItems < 0 {
			fail("%s: min_items is negative", at)
		}
		if f.MinItems != nil && f.MaxItems != nil && *f.MinItems > *f.MaxItems {
			fail("%s: min_items is greater than max_items", at)
		}
	} else {
		if f.Item != nil {
			fail("%s: item only applies to list", at)
		}
		if f.MinItems != nil || f.MaxItems != nil {
			fail("%s: min_items and max_items only apply to list", at)
		}
	}

	if f.Type == TypeStruct {
		if len(f.Fields) == 0 {
			fail("%s: struct needs fields", at)
		}
		names := map[string]bool{}
		for _, sub := range f.Fields {
			subAt := fmt.Sprintf("%s field %q", at, sub.Name)
			if sub.Name == "" {
				fail("%s: struct fields need a name", subAt)
			}
			if names[sub.Name] {
				fail("%s: duplicate", subAt)
			}
			names[sub.Name] = true
			sub.validate(subAt, false, fail)
		}
	} else if len(f.Fields) > 0 {
		fail("%s: fields only apply to struct", at)
	}
}

func number(at, key string, raw any, typ Type, fail func(string, ...any)) *float64 {
	if raw == nil {
		return nil
	}
	var v float64
	switch n := raw.(type) {
	case int:
		v = float64(n)
	case float64:
		if typ == TypeInt {
			fail("%s: %s must be a whole number for int", at, key)
		}
		v = n
	default:
		fail("%s: %s must be a number, got %v", at, key, raw)
		return nil
	}
	return &v
}

func duration(at, key string, raw any, fail func(string, ...any)) *time.Duration {
	if raw == nil {
		return nil
	}
	str, ok := raw.(string)
	if !ok {
		fail("%s: %s must be a duration string such as \"30s\", got %v", at, key, raw)
		return nil
	}
	d, err := time.ParseDuration(str)
	if err != nil {
		fail("%s: %s: %v", at, key, err)
		return nil
	}
	return &d
}

func knownUnit(unit string) bool {
	return slices.Contains(Units, unit)
}
