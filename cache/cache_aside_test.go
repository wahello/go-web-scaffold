package cache

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/stretchr/testify/require"
)

const (
	dummy1 = "我能吞下玻璃而不伤到身体"
	dummy2 = "滚滚长江东逝水"
)

func TestCache_Compresses(t *testing.T) {
	tests := []struct {
		key            string
		payload        string
		wantCompressed bool
		wantErr        bool
	}{
		{
			key:            "no-compress-1",
			payload:        dummy1,
			wantCompressed: false,
			wantErr:        false,
		},
		{
			key:            "no-compress-2",
			payload:        dummy2,
			wantCompressed: false,
			wantErr:        false,
		},
		{
			key:            "compress-1",
			payload:        strings.Repeat(dummy1, 1000),
			wantCompressed: true,
			wantErr:        false,
		},
		{
			key:            "compress-2",
			payload:        strings.Repeat(dummy2, 1000),
			wantCompressed: true,
			wantErr:        false,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if err := cache.UpdateBytes(ctx, tt.key, []byte(tt.payload), 0); (err != nil) != tt.wantErr {
				t.Errorf("UpdateBytes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})

		rawPlayback, err := cache.Redis.Get(ctx, tt.key).Bytes()
		require.NoError(t, err)
		require.Equal(t, tt.wantCompressed, isGzipped(rawPlayback))
		if !tt.wantCompressed {
			require.Equal(t, tt.payload, string(rawPlayback))
		}

		playback, err := cache.ReadBytes(ctx, tt.key)
		require.NoError(t, err)
		require.Equal(t, tt.payload, string(playback))
	}
}

func TestCURD(t *testing.T) {
	const key = "cache-test-vb56ae3"

	type Payload struct {
		A int
		B string
		C time.Time
		D float64
		E []int32
	}

	var payload = Payload{
		A: 1,
		B: "2",
		C: time.Now(),
		D: 4.5,
		E: []int32{5, 6, 8, 7, 9},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var got Payload
	err := cache.Read(ctx, key, &got)
	require.True(t, errors.Is(err, redis.Nil))

	err = cache.Update(ctx, key, payload, 0)
	require.NoError(t, err)

	result, err := cache.Redis.TTL(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, time.Duration(-1), result)

	err = cache.Read(ctx, key, &got)
	require.NoError(t, err)
	require.True(t, payload.C.Equal(got.C))

	// deepEqual does not apply to time.Time
	got.C = payload.C
	require.Equal(t, payload, got)

	err = cache.Revoke(ctx, key)
	require.NoError(t, err)
	err = cache.Read(ctx, key, &got)
	require.True(t, errors.Is(err, redis.Nil))

	err = cache.Update(ctx, key, payload, 20*time.Second)
	require.NoError(t, err)

	result, err = cache.Redis.TTL(ctx, key).Result()
	require.NoError(t, err)
	require.True(t, 20*time.Second >= result)
	require.True(t, 20*time.Second-result < 2*time.Second)

	err = cache.RevokeByPattern(ctx, "cache-*")
	require.NoError(t, err)
	err = cache.Read(ctx, key, &got)
	require.True(t, errors.Is(err, redis.Nil))
}
