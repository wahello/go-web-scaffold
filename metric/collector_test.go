package metric

import (
	"reflect"
	"testing"

	"github.com/segmentio/stats/v4"
)

func Test_newStatsKey(t *testing.T) {
	tests := []struct {
		testName string
		prefix   string
		name     string
		wantKey  stats.Key
	}{
		{
			testName: "empty",
			prefix:   "",
			name:     "",
			wantKey: stats.Key{
				Measure: "",
				Field:   "",
			},
		},
		{
			testName: "empty prefix",
			prefix:   "",
			name:     "abc",
			wantKey: stats.Key{
				Measure: "",
				Field:   "abc",
			},
		},
		{
			testName: "empty prefix with dot",
			prefix:   "",
			name:     "abc.def",
			wantKey: stats.Key{
				Measure: "abc",
				Field:   "def",
			},
		},
		{
			testName: "empty prefix with many dots",
			prefix:   "",
			name:     "abc.def.efg",
			wantKey: stats.Key{
				Measure: "abc.def",
				Field:   "efg",
			},
		},
		{
			testName: "has prefix",
			prefix:   "123",
			name:     "abc",
			wantKey: stats.Key{
				Measure: "123",
				Field:   "abc",
			},
		},
		{
			testName: "has prefix with dot",
			prefix:   "123",
			name:     "abc.def",
			wantKey: stats.Key{
				Measure: "123.abc",
				Field:   "def",
			},
		},
		{
			testName: "has prefix with many dots",
			prefix:   "123",
			name:     "abc.def.efg",
			wantKey: stats.Key{
				Measure: "123.abc.def",
				Field:   "efg",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			if gotKey := newStatsKey(tt.prefix, tt.name); !reflect.DeepEqual(gotKey, tt.wantKey) {
				t.Errorf("newStatsKey() = %v, want %v", gotKey, tt.wantKey)
			}
		})
	}
}
