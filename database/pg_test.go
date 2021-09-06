package database

import (
	"context"
	"testing"
	"time"

	"go.uber.org/atomic"

	"github.com/go-pg/pg/v10"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDB_WatchAndNotify(t *testing.T) {
	const (
		topic1 = "test:topic1"
		topic2 = "test:topic2"

		payload1 = "test_payload_123489rds"
		payload2 = "test_payload_1cds1aq18"
	)

	var (
		err           error
		cb1, cb2, cb3 atomic.Int64
		finished      atomic.Bool
	)

	t.Cleanup(func() {
		finished.Store(true)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = db.Watch(ctx, func(_ context.Context, notify pg.Notification) {
		if finished.Load() {
			return
		}
		cb1.Add(1)
		assert.Equal(t, topic1, notify.Channel)
		assert.Equal(t, payload1, notify.Payload)
	}, topic1)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// 测试发送和接收
	err = db.Notify(ctx, topic1, payload1)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	require.Equal(t, int64(1), cb1.Load())

	err = db.Watch(ctx, func(_ context.Context, notify pg.Notification) {
		if finished.Load() {
			return
		}
		cb2.Add(1)
		assert.Equal(t, topic2, notify.Channel)
		assert.Equal(t, payload2, notify.Payload)
	}, topic2)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	err = db.Notify(ctx, topic1, payload1)
	require.NoError(t, err)

	err = db.Notify(ctx, topic2, payload2)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	require.Equal(t, int64(1), cb2.Load())
	require.Equal(t, int64(2), cb1.Load())

	err = db.Watch(ctx, func(_ context.Context, notify pg.Notification) {
		if finished.Load() {
			return
		}
		cb3.Add(1)
		if notify.Channel != topic1 &&
			notify.Channel != topic2 {
			t.Errorf("unexpected topic %q", notify.Channel)
		}
		if notify.Payload != payload1 &&
			notify.Payload != payload2 {
			t.Errorf("unexpected payload %q", notify.Payload)
		}
	}, topic2, topic1)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	err = db.Notify(ctx, topic1, payload1)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	require.Equal(t, int64(1), cb3.Load())

	err = db.Notify(ctx, topic2, payload2)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	require.Equal(t, int64(2), cb3.Load())
	require.Equal(t, int64(2), cb2.Load())
	require.Equal(t, int64(3), cb1.Load())
}
