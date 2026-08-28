package main

import "testing"

func TestFilterPartitionsByName(t *testing.T) {
	partitions := []PartitionItem{
		{Name: "boot"},
		{Name: "init_boot"},
		{Name: "system"},
		{Name: "vendor_boot"},
	}
	tests := []struct {
		query string
		want  []string
	}{
		{query: "", want: []string{"boot", "init_boot", "system", "vendor_boot"}},
		{query: "boot", want: []string{"boot", "init_boot", "vendor_boot"}},
		{query: "INIT", want: []string{"init_boot"}},
		{query: " system ", want: []string{"system"}},
		{query: "missing", want: nil},
	}
	for _, test := range tests {
		got := filterPartitions(partitions, test.query)
		if len(got) != len(test.want) {
			t.Fatalf("query %q returned %d items, want %d", test.query, len(got), len(test.want))
		}
		for index, name := range test.want {
			if got[index].Name != name {
				t.Fatalf("query %q item %d = %q, want %q", test.query, index, got[index].Name, name)
			}
		}
	}
}

func TestParseThreadCount(t *testing.T) {
	for _, input := range []string{"1", "4", "64", " 8 "} {
		if _, err := parseThreadCount(input); err != nil {
			t.Fatalf("parseThreadCount(%q) returned %v", input, err)
		}
	}
	for _, input := range []string{"", "0", "65", "-1", "abc", "4.5"} {
		if _, err := parseThreadCount(input); err == nil {
			t.Fatalf("parseThreadCount(%q) unexpectedly succeeded", input)
		}
	}
}
