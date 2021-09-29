package cache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/klauspost/compress/gzip"
)

// compressThreshold gzip content larger than 4 KiB
const compressThreshold = 4 * 1024

// Pool for gzip writers and readers
var (
	gwriters sync.Pool
	greaders sync.Pool
)

/*
Cache Aside Pattern

* hit: read from cache first, and return it directly when hitting.
* miss: when cache misses, read it from origin(e.g. database), put it into cache, return it.
* update: after origin updates, revoke(delete) the cache.
*/

// Read reads cache content which is set by Update
//
// dest must be a pointer type.
//
// when err is nil, a valid cache is obtained.
func (red *Cache) Read(ctx context.Context, key string, dest interface{}) (err error) {
	raw, err := red.ReadBytes(ctx, key)
	if err != nil {
		err = fmt.Errorf("ReadBytes: %w", err)
		return
	}

	err = msgpack.Unmarshal(raw, dest)
	if err != nil {
		err = fmt.Errorf("msgpack decoding: %w", err)
		return
	}

	return
}

// Update writes cache content which can be read by Read()
//
// If payload is a string, use UpdateBytes()
//
// Set durationSeconds to 0 to make this key never expires
func (red *Cache) Update(ctx context.Context, key string, payload interface{}, expiration time.Duration) (err error) {
	buf, err := msgpack.Marshal(payload)
	if err != nil {
		err = fmt.Errorf("msgpack encode: %w", err)
		return
	}

	err = red.UpdateBytes(ctx, key, buf, expiration)
	if err != nil {
		err = fmt.Errorf("UpdateBytes: %w", err)
		return
	}

	return
}

// Revoke deletes cache by key
func (red *Cache) Revoke(ctx context.Context, key ...string) (err error) {
	err = red.Redis.Unlink(ctx, key...).Err()
	if err != nil {
		err = fmt.Errorf("redis Unlink: %w", err)
		return
	}

	return
}

// RevokeByPattern deletes keys that matched by patten
//
// matching rule: https://redis.io/commands/keys
//
// This command does not guarantee atomic, but works well on
// large key space.
func (red *Cache) RevokeByPattern(ctx context.Context, patten string) (err error) {
	var (
		keys []string
		i    int
	)

	iter := red.Redis.Scan(ctx, 0, patten, 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())

		// for every 1000 keys, do a round of delete
		i++
		if i > 1000 {
			i = 0

			err = red.Redis.Unlink(ctx, keys...).Err()
			if err != nil {
				err = fmt.Errorf("redis Unlink in scan loop: %w", err)
				return
			}

			// reuse allocated memory
			keys = keys[:0]
		}
	}
	err = iter.Err()
	if err != nil {
		err = fmt.Errorf("iter.Err: %w", err)
		return
	}

	if len(keys) > 0 {
		err = red.Redis.Unlink(ctx, keys...).Err()
		if err != nil {
			err = fmt.Errorf("redis Unlink after scan finish: %w", err)
			return
		}
	}

	return
}

// shouldCompress decides whether or not to compress b
func shouldCompress(b []byte) bool {
	// compress content larger than compressThreshold
	// content should not be gzipped already
	return len(b) > compressThreshold && !isGzipped(b)
}

// isGzipped tests if content is gzipped
func isGzipped(b []byte) bool {
	const (
		gzipID1     = 0x1f
		gzipID2     = 0x8b
		gzipDeflate = 8
	)

	if len(b) < 3 {
		return false
	}

	if b[0] != gzipID1 || b[1] != gzipID2 || b[2] != gzipDeflate {
		return false
	}

	return true
}

// ReadBytes read bytes cache from cache,
// decompress if in need.
func (red *Cache) ReadBytes(ctx context.Context, key string) (b []byte, err error) {
	raw, err := red.Redis.Get(ctx, key).Bytes()
	if err != nil {
		err = fmt.Errorf("redis GET: %w", err)
		return
	}

	if !isGzipped(raw) {
		b = raw
		return
	}

	reader, _ := greaders.Get().(*gzip.Reader)
	if reader == nil {
		reader, err = gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			err = fmt.Errorf("gzip.NewReader: %w", err)
			return
		}
	} else {
		err = reader.Reset(bytes.NewReader(raw))
		if err != nil {
			err = fmt.Errorf("reader.Reset: %w", err)
			return
		}
	}
	defer greaders.Put(reader)
	defer reader.Close()

	var dest bytes.Buffer

	_, err = io.Copy(&dest, reader)
	if err != nil {
		err = fmt.Errorf("io.Copy: %w", err)
		return
	}

	b = dest.Bytes()
	return
}

// UpdateBytes update cache with bytes payload
//
// content may be compressed,
// which can be fetched with ReadBytes
func (red *Cache) UpdateBytes(ctx context.Context, key string, payload []byte, expiration time.Duration) (err error) {
	if !shouldCompress(payload) {
		return red.Redis.Set(ctx, key, payload, expiration).Err()
	}

	var buf bytes.Buffer

	writer, _ := gwriters.Get().(*gzip.Writer)
	if writer == nil {
		writer = gzip.NewWriter(&buf)
	} else {
		writer.Reset(&buf)
	}
	defer gwriters.Put(writer)

	_, err = writer.Write(payload)
	if err != nil {
		err = fmt.Errorf("writer.Write: %w", err)
		return
	}

	// flush gzipped content
	err = writer.Close()
	if err != nil {
		err = fmt.Errorf("gzip writer.Close: %w", err)
		return
	}
	err = red.Redis.Set(ctx, key, buf.Bytes(), expiration).Err()
	if err != nil {
		err = fmt.Errorf("redis SET gzipped: %w", err)
		return
	}

	return
}
