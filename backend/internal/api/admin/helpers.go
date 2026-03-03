package admin

import (
	"encoding/json"
	"io"
)

// decodeJSON decodes JSON from a reader into dest.
func decodeJSON(r io.Reader, dest any) error {
	return json.NewDecoder(r).Decode(dest)
}
