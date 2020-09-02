package json

import (
	"errors"

	"github.com/json-iterator/go"
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
	// Valid reports whether data is a valid JSON encoding.
	Valid = json.Valid
	// Marshal returns the JSON encoding of v.
	Marshal = json.Marshal
	// Unmarshal parses the JSON-encoded data and stores the result in the value pointed to by v.
	Unmarshal = json.Unmarshal
	// MarshalIndent is like Marshal but applies Indent to format the output.
	MarshalIndent = json.MarshalIndent
	// NewDecoder returns a new decoder that reads from r.
	NewDecoder = json.NewDecoder
	// NewEncoder returns a new encoder that writes to w.
	NewEncoder = json.NewEncoder
)

// RawMessage is a raw encoded JSON value.
// It implements Marshaler and Unmarshaler and can
// be used to delay JSON decoding or precompute a JSON encoding.
type RawMessage []byte

// MarshalJSON returns m as the JSON encoding of m.
func (m RawMessage) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	return m, nil
}

// UnmarshalJSON sets *m to a copy of data.
func (m *RawMessage) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.New("json.RawMessage: UnmarshalJSON on nil pointer")
	}
	*m = append((*m)[0:0], data...)
	return nil
}
