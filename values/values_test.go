package values

import (
	"strings"
	"testing"
	"time"

	clientsapi "github.com/jimjibone/woodhouse-api/go/v2/clients"
)

func TestTimeRoundTrip(t *testing.T) {
	now := time.Unix(1_800_000_000, 123)
	if got := TimeFromPb(TimeToPb(now)); !got.Equal(now) {
		t.Fatalf("round trip: %v != %v", got, now)
	}
	if !TimeFromPb(TimeToPb(time.Time{})).IsZero() {
		t.Fatal("zero time not preserved")
	}
}

func TestValidate(t *testing.T) {
	min, max, step := int64(0), int64(100), uint64(5)
	intSpec := &clientsapi.Spec{Kind: &clientsapi.Spec_Int{Int: &clientsapi.IntSpec{Min: &min, Max: &max, Step: &step}}}
	enumSpec := &clientsapi.Spec{Kind: &clientsapi.Spec_Enum{Enum: &clientsapi.EnumSpec{Options: []string{"a", "b"}}}}
	hsOnly := &clientsapi.Spec{Kind: &clientsapi.Spec_Color{Color: &clientsapi.ColorSpec{HueSat: true}}}

	tests := []struct {
		name string
		spec *clientsapi.Spec
		v    *clientsapi.Value
		want string // "" means valid
	}{
		{"int ok", intSpec, Int(55), ""},
		{"int step", intSpec, Int(52), "step"},
		{"int max", intSpec, Int(105), "above max"},
		{"kind", intSpec, Float(5), "got float, want int"},
		{"enum ok", enumSpec, Enum("b"), ""},
		{"enum option", enumSpec, Enum("c"), "not one of"},
		{"color hs", hsOnly, HueSat(120, 50), ""},
		{"color xy", hsOnly, XY(0.3, 0.3), "xy not supported"},
		{"color range", hsOnly, HueSat(400, 50), "out of range"},
		{"no value", intSpec, nil, "no value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.spec, tt.v)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("valid value rejected: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}
