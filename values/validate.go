package values

import (
	"errors"
	"fmt"
	"slices"

	clientsapi "github.com/jimjibone/woodhouse-api/go/v2/clients"
)

// Validate checks that a value matches a spec: same kind and within its
// constraints. Steps are checked for int and duration (relative to min, or 0
// without one), not for float.
func Validate(spec *clientsapi.Spec, v *clientsapi.Value) error {
	if spec == nil || spec.Kind == nil {
		return errors.New("no spec")
	}
	if !IsSet(v) {
		return errors.New("no value")
	}
	switch s := spec.Kind.(type) {
	case *clientsapi.Spec_Bool:
		return kind(v, "bool")
	case *clientsapi.Spec_Text:
		return kind(v, "text")
	case *clientsapi.Spec_Image:
		return kind(v, "image")
	case *clientsapi.Spec_Time:
		if err := kind(v, "time"); err != nil {
			return err
		}
		if n := v.GetTime().GetNanos(); n < 0 || n > 999_999_999 {
			return fmt.Errorf("nanos %d out of range", n)
		}
		return nil
	case *clientsapi.Spec_Int:
		if err := kind(v, "int"); err != nil {
			return err
		}
		return checkInt(v.GetInt(), s.Int.Min, s.Int.Max, s.Int.Step)
	case *clientsapi.Spec_Duration:
		if err := kind(v, "duration"); err != nil {
			return err
		}
		return checkInt(v.GetDuration(), s.Duration.Min, s.Duration.Max, s.Duration.Step)
	case *clientsapi.Spec_Float:
		if err := kind(v, "float"); err != nil {
			return err
		}
		f := v.GetFloat()
		if s.Float.Min != nil && f < *s.Float.Min {
			return fmt.Errorf("%v is below min %v", f, *s.Float.Min)
		}
		if s.Float.Max != nil && f > *s.Float.Max {
			return fmt.Errorf("%v is above max %v", f, *s.Float.Max)
		}
		return nil
	case *clientsapi.Spec_Color:
		if err := kind(v, "color"); err != nil {
			return err
		}
		c := v.GetColor()
		if c.GetHueSat() == nil && c.GetXy() == nil {
			return errors.New("color has neither hue_sat nor xy")
		}
		if hs := c.GetHueSat(); hs != nil {
			if !s.Color.HueSat {
				return errors.New("hue_sat not supported")
			}
			if hs.Hue < 0 || hs.Hue > 360 || hs.Sat < 0 || hs.Sat > 100 {
				return fmt.Errorf("hue_sat %v/%v out of range", hs.Hue, hs.Sat)
			}
		}
		if xy := c.GetXy(); xy != nil {
			if !s.Color.Xy {
				return errors.New("xy not supported")
			}
			if xy.X < 0 || xy.X > 1 || xy.Y < 0 || xy.Y > 1 {
				return fmt.Errorf("xy %v/%v out of range", xy.X, xy.Y)
			}
		}
		return nil
	case *clientsapi.Spec_Enum:
		if err := kind(v, "enum"); err != nil {
			return err
		}
		if len(s.Enum.Options) > 0 && !slices.Contains(s.Enum.Options, v.GetEnum()) {
			return fmt.Errorf("%q is not one of %q", v.GetEnum(), s.Enum.Options)
		}
		return nil
	case *clientsapi.Spec_List:
		if err := kind(v, "list"); err != nil {
			return err
		}
		items := v.GetList().GetItems()
		if s.List.MinItems != nil && uint32(len(items)) < *s.List.MinItems {
			return fmt.Errorf("%d items, min %d", len(items), *s.List.MinItems)
		}
		if s.List.MaxItems != nil && uint32(len(items)) > *s.List.MaxItems {
			return fmt.Errorf("%d items, max %d", len(items), *s.List.MaxItems)
		}
		for i, item := range items {
			if err := Validate(s.List.Item, item); err != nil {
				return fmt.Errorf("item %d: %w", i, err)
			}
		}
		return nil
	case *clientsapi.Spec_Struct:
		if err := kind(v, "struct"); err != nil {
			return err
		}
		return ValidateFields(s.Struct.Fields, v.GetStruct().GetFields())
	}
	return fmt.Errorf("unknown spec kind %T", spec.Kind)
}

// ValidateFields checks named values (command params, event fields, struct
// fields) against their specs: no unknown or duplicate IDs, every required
// field present, and each value valid.
func ValidateFields(specs []*clientsapi.FieldSpec, fields []*clientsapi.FieldValue) error {
	byID := make(map[string]*clientsapi.FieldSpec, len(specs))
	for _, s := range specs {
		byID[s.GetId()] = s
	}
	seen := make(map[string]bool, len(fields))
	var errs []error
	for _, f := range fields {
		id := f.GetId()
		spec, ok := byID[id]
		if !ok {
			errs = append(errs, fmt.Errorf("%q: unknown", id))
			continue
		}
		if seen[id] {
			errs = append(errs, fmt.Errorf("%q: duplicate", id))
			continue
		}
		seen[id] = true
		if err := Validate(spec.GetSpec(), f.GetValue()); err != nil {
			errs = append(errs, fmt.Errorf("%q: %w", id, err))
		}
	}
	for _, s := range specs {
		if s.GetRequired() && !seen[s.GetId()] {
			errs = append(errs, fmt.Errorf("%q: required", s.GetId()))
		}
	}
	return errors.Join(errs...)
}

func kind(v *clientsapi.Value, want string) error {
	if got := Kind(v); got != want {
		return fmt.Errorf("got %s, want %s", got, want)
	}
	return nil
}

func checkInt(v int64, min, max *int64, step *uint64) error {
	if min != nil && v < *min {
		return fmt.Errorf("%d is below min %d", v, *min)
	}
	if max != nil && v > *max {
		return fmt.Errorf("%d is above max %d", v, *max)
	}
	if step != nil && *step > 1 {
		base := int64(0)
		if min != nil {
			base = *min
		}
		if (v-base)%int64(*step) != 0 {
			return fmt.Errorf("%d is not a multiple of step %d from %d", v, *step, base)
		}
	}
	return nil
}
