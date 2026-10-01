package services

import "encoding/json"

// marshalJSON wraps encoding/json.Marshal for internal use.
func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
