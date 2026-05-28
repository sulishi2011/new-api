package model

import (
	"errors"
	"testing"
)

func TestIsPostgresRecoveryConflict(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "recovery conflict",
			err:  errors.New("ERROR: canceling statement due to conflict with recovery (SQLSTATE 40001)"),
			want: true,
		},
		{
			name: "serialization failure",
			err:  errors.New("ERROR: could not serialize access due to concurrent update (SQLSTATE 40001)"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPostgresRecoveryConflict(tt.err); got != tt.want {
				t.Fatalf("isPostgresRecoveryConflict() = %v, want %v", got, tt.want)
			}
		})
	}
}
