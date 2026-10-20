package main

import "testing"

func TestSortAscendingArg(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		want    bool
		wantErr bool
	}{
		{name: "default is descending", args: map[string]any{}, want: false},
		{name: "asc", args: map[string]any{"sort_direction": "asc"}, want: true},
		{name: "desc", args: map[string]any{"sort_direction": "desc"}, want: false},
		{name: "unknown value", args: map[string]any{"sort_direction": "sideways"}, wantErr: true},
		{name: "empty value", args: map[string]any{"sort_direction": ""}, wantErr: true},
		{name: "wrong type", args: map[string]any{"sort_direction": 1.0}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sortAscendingArg(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("sortAscendingArg() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("sortAscendingArg() = %v, want %v", got, tt.want)
			}
		})
	}
}
