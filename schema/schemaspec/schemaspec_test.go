package schemaspec

import (
	"strings"
	"testing"
	"time"

	clientsapi "github.com/jimjibone/woodhouse-api/go/v2/clients"
	"github.com/jimjibone/woodhouse-api/schema"
	"github.com/jimjibone/woodhouse-api/values"
)

// Every field, command and event in the schema converts to a spec.
func TestSpecFromSchemaCoversSchema(t *testing.T) {
	s := schema.MustLoad()
	for _, srv := range s.Services {
		var all []string
		for _, f := range srv.Features {
			all = append(all, f.Name)
		}
		for _, a := range srv.Attributes {
			if SpecFromSchema(a).Kind == nil {
				t.Errorf("%s.%s: no spec", srv.Name, a.Name)
			}
		}
		for _, c := range srv.Commands {
			CommandSpecFromSchema(c, all)
		}
		for _, e := range srv.Events {
			EventSpecFromSchema(e, all)
		}
	}
}

func TestFeaturesFilterFields(t *testing.T) {
	set := schema.MustLoad().Service("light").Command("set")
	ids := func(specs []*clientsapi.FieldSpec) string {
		var out []string
		for _, s := range specs {
			out = append(out, s.Id)
		}
		return strings.Join(out, ",")
	}
	if got := ids(CommandSpecFromSchema(set, nil).Params); got != "on,transition" {
		t.Errorf("no features: %s", got)
	}
	if got := ids(CommandSpecFromSchema(set, []string{"brightness", "xy"}).Params); got != "on,brightness,color,transition" {
		t.Errorf("brightness+xy: %s", got)
	}
}

// Specs built from the schema validate values as the schema says.
func TestValidateCommandParams(t *testing.T) {
	s := schema.MustLoad()
	set := CommandSpecFromSchema(s.Service("light").Command("set"), []string{"brightness", "color_temp", "hs"})

	ok := []*clientsapi.FieldValue{
		values.Field("on", values.Bool(true)),
		values.Field("brightness", values.Int(50)),
		values.Field("color_temp", values.Int(2700)),
		values.Field("transition", values.Duration(2*time.Second)),
	}
	if err := values.ValidateFields(set.Params, ok); err != nil {
		t.Fatalf("valid params rejected: %v", err)
	}

	tests := []struct {
		name   string
		fields []*clientsapi.FieldValue
		want   string
	}{
		{"wrong kind", []*clientsapi.FieldValue{values.Field("on", values.Int(1))}, "got int, want bool"},
		{"above max", []*clientsapi.FieldValue{values.Field("brightness", values.Int(101))}, "above max"},
		{"unknown", []*clientsapi.FieldValue{values.Field("bogus", values.Bool(true))}, "unknown"},
		{"duplicate", []*clientsapi.FieldValue{values.Field("on", values.Bool(true)), values.Field("on", values.Bool(false))}, "duplicate"},
		{"step", []*clientsapi.FieldValue{values.Field("transition", values.Duration(1500 * time.Millisecond))}, "step"},
		{"xy not enabled", []*clientsapi.FieldValue{values.Field("color", values.XY(0.3, 0.3))}, "xy not supported"},
		{"unset value", []*clientsapi.FieldValue{values.Field("on", nil)}, "no value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := values.ValidateFields(set.Params, tt.fields)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateSchedule(t *testing.T) {
	cmd := CommandSpecFromSchema(schema.MustLoad().Service("thermostat").Command("set_schedule"), []string{"schedule"})
	entry := func(days []string, start time.Duration, setpoint float64) *clientsapi.Value {
		var ds []*clientsapi.Value
		for _, d := range days {
			ds = append(ds, values.Enum(d))
		}
		return values.Struct(
			values.Field("days", values.List(ds...)),
			values.Field("start", values.Duration(start)),
			values.Field("setpoint", values.Float(setpoint)),
		)
	}
	field := func(v *clientsapi.Value) []*clientsapi.FieldValue {
		return []*clientsapi.FieldValue{values.Field("schedule", v)}
	}

	good := values.List(entry([]string{"mon", "tue"}, 7*time.Hour, 20.5), entry([]string{"sat"}, 9*time.Hour, 19))
	if err := values.ValidateFields(cmd.Params, field(good)); err != nil {
		t.Fatalf("valid schedule rejected: %v", err)
	}
	if err := values.ValidateFields(cmd.Params, field(values.List(entry([]string{"funday"}, 7*time.Hour, 20)))); err == nil || !strings.Contains(err.Error(), "funday") {
		t.Fatalf("bad day accepted: %v", err)
	}
	if err := values.ValidateFields(cmd.Params, field(values.List(entry(nil, 7*time.Hour, 20)))); err == nil || !strings.Contains(err.Error(), "min 1") {
		t.Fatalf("empty days accepted: %v", err)
	}
	if err := values.ValidateFields(cmd.Params, nil); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("missing schedule accepted: %v", err)
	}
}
