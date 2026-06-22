package helps

import (
	"github.com/tidwall/sjson"
)

// DeleteJSONField removes the given JSON path from body.
// It returns body unchanged if key is empty or body is empty, or if deletion fails.
func DeleteJSONField(body []byte, key string) []byte {
	if key == "" || len(body) == 0 {
		return body
	}
	updated, err := sjson.DeleteBytes(body, key)
	if err != nil {
		return body
	}
	return updated
}
