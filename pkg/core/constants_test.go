package core

import "testing"

func TestScrollStrategyValues(t *testing.T) {
	cases := []struct {
		name string
		got  ScrollStrategy
		want string
	}{
		{"window", ScrollStrategyWindow, ""},
		{"generic-list", ScrollStrategyGenericList, "generic-list"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.got) != tc.want {
				t.Errorf("ScrollStrategy %s = %q want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestScrollStrategy_TypeDistinct(t *testing.T) {
	// Ensure the two constants are distinct.
	if ScrollStrategyWindow == ScrollStrategyGenericList {
		t.Error("ScrollStrategyWindow and ScrollStrategyGenericList must be distinct")
	}
}
