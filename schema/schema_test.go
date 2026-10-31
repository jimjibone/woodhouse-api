package schema

import (
	"strings"
	"testing"

	clientsapi "github.com/jimjibone/woodhouse-api/go/v2/clients"
)

func TestEmbeddedSchemaIsValid(t *testing.T) {
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if s.Version <= 0 {
		t.Fatalf("version = %d", s.Version)
	}
}

// The proto Service.ServiceType enum is hand-written; it must list exactly the
// schema's service types.
func TestServiceTypesMatchProto(t *testing.T) {
	s := MustLoad()
	inSchema := map[string]bool{}
	for _, srv := range s.Services {
		inSchema[srv.ProtoName()] = true
		if _, ok := clientsapi.Service_ServiceType_value[srv.ProtoName()]; !ok {
			t.Errorf("service %q has no Service.ServiceType %s in the proto", srv.Name, srv.ProtoName())
		}
	}
	for name := range clientsapi.Service_ServiceType_value {
		if name == "UNDEFINED" {
			continue
		}
		if !inSchema[name] {
			t.Errorf("Service.ServiceType %s is not in the schema", name)
		}
	}
}

func TestUnitsMatchProto(t *testing.T) {
	inSchema := map[string]bool{}
	for _, u := range Units {
		inSchema[UnitProtoName(u)] = true
		if _, ok := clientsapi.Unit_value[UnitProtoName(u)]; !ok {
			t.Errorf("unit %q has no %s in the proto", u, UnitProtoName(u))
		}
	}
	for name := range clientsapi.Unit_value {
		if name == "UNIT_UNDEFINED" {
			continue
		}
		if !inSchema[name] {
			t.Errorf("proto unit %s is not in Units", name)
		}
	}
}

func TestParsedConstraints(t *testing.T) {
	s := MustLoad()
	light := s.Service("light")
	if light == nil {
		t.Fatal("no light service")
	}
	ct := light.Attribute("color_temp")
	if ct == nil || ct.Min == nil || *ct.Min != 1000 || *ct.Max != 10000 || ct.Unit != "kelvin" {
		t.Fatalf("light.color_temp constraints not parsed: %+v", ct)
	}
	if light.Attribute("on").Perms != PermsRW {
		t.Fatal("light.on should be rw")
	}
	if light.Attribute("color_mode").Perms != PermsRO {
		t.Fatal("perms should default to ro")
	}
	tr := light.Command("set").Params[4]
	if tr.Name != "transition" || tr.MaxDuration == nil || tr.MaxDuration.Seconds() != 300 {
		t.Fatalf("light.set transition not parsed: %+v", tr)
	}

	sched := s.Service("thermostat").Attribute("schedule")
	if sched.Type != TypeList || sched.Item == nil || sched.Item.Type != TypeStruct || len(sched.Item.Fields) != 3 {
		t.Fatalf("thermostat.schedule not parsed: %+v", sched)
	}
	if sched.Item.Fields[1].MaxDuration == nil || sched.Item.Fields[1].MaxDuration.Hours() != 24 {
		t.Fatal("schedule start max not parsed")
	}
}

func TestInvalidSchemas(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"unknown key", `
version: 1
services:
  - { name: a, desc: x, bogus: 1 }`, "bogus"},
		{"duplicate service", `
version: 1
services:
  - { name: a, desc: x }
  - { name: a, desc: x }`, "duplicate service"},
		{"bad type", `
version: 1
services:
  - name: a
    desc: x
    attributes:
      - { name: v, type: number }`, "invalid type"},
		{"unknown feature", `
version: 1
services:
  - name: a
    desc: x
    attributes:
      - { name: v, type: bool, features: [nope] }`, "unknown feature"},
		{"unused feature", `
version: 1
services:
  - name: a
    desc: x
    features: [{ name: f }]`, "not used"},
		{"required and features", `
version: 1
services:
  - name: a
    desc: x
    features: [{ name: f }]
    attributes:
      - { name: v, type: bool, required: true, features: [f] }`, "mutually exclusive"},
		{"unit on text", `
version: 1
services:
  - name: a
    desc: x
    attributes:
      - { name: v, type: text, unit: celsius }`, "unit only applies"},
		{"unknown unit", `
version: 1
services:
  - name: a
    desc: x
    attributes:
      - { name: v, type: float, unit: furlongs }`, "unknown unit"},
		{"fractional int", `
version: 1
services:
  - name: a
    desc: x
    attributes:
      - { name: v, type: int, min: 0.5 }`, "whole number"},
		{"bad duration", `
version: 1
services:
  - name: a
    desc: x
    attributes:
      - { name: v, type: duration, max: 5 }`, "duration string"},
		{"list without item", `
version: 1
services:
  - name: a
    desc: x
    attributes:
      - { name: v, type: list }`, "needs an item"},
		{"empty struct", `
version: 1
services:
  - name: a
    desc: x
    attributes:
      - { name: v, type: struct }`, "needs fields"},
		{"nested feature", `
version: 1
services:
  - name: a
    desc: x
    features: [{ name: f }]
    attributes:
      - { name: v, type: list, features: [f], item: { type: bool, features: [f] } }`, "top-level"},
		{"perms on param", `
version: 1
services:
  - name: a
    desc: x
    commands:
      - name: c
        params:
          - { name: p, type: bool, perms: rw }`, "perms only apply"},
		{"write-only perms", `
version: 1
services:
  - name: a
    desc: x
    attributes:
      - { name: v, type: bool, perms: wo }`, "invalid perms"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}
