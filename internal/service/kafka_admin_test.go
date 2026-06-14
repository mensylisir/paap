package service

import (
	"testing"

	"github.com/segmentio/kafka-go"
)

func TestParseKafkaOffset(t *testing.T) {
	cases := []struct {
		name      string
		value     string
		want      int64
		wantError bool
	}{
		{name: "empty uses first offset", value: "", want: kafka.FirstOffset},
		{name: "first alias", value: "first", want: kafka.FirstOffset},
		{name: "latest alias", value: "latest", want: kafka.LastOffset},
		{name: "numeric offset", value: "42", want: 42},
		{name: "negative offset rejected", value: "-1", wantError: true},
		{name: "invalid offset rejected", value: "abc", wantError: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok, err := parseKafkaOffset(tc.value)
			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !ok {
				t.Fatalf("expected parsed offset")
			}
			if got != tc.want {
				t.Fatalf("offset = %d, want %d", got, tc.want)
			}
		})
	}
}
