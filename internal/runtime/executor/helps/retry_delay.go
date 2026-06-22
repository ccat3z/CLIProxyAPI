package helps

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// ParseRetryDelay extracts the retry delay from a Google API 429 error response.
// The error response contains a RetryInfo.retryDelay field in the format "0.847655010s".
// Returns the parsed duration or an error if it cannot be determined.
func ParseRetryDelay(errorBody []byte) (*time.Duration, error) {
	// Try to parse the retryDelay from the error response
	// Format: error.details[].retryDelay where @type == "type.googleapis.com/google.rpc.RetryInfo"
	details := gjson.GetBytes(errorBody, "error.details")
	if details.Exists() && details.IsArray() {
		for _, detail := range details.Array() {
			typeVal := detail.Get("@type").String()
			if typeVal == "type.googleapis.com/google.rpc.RetryInfo" {
				retryDelay := detail.Get("retryDelay").String()
				if retryDelay != "" {
					// Parse duration string like "0.847655010s"
					duration, err := time.ParseDuration(retryDelay)
					if err != nil {
						return nil, fmt.Errorf("failed to parse duration")
					}
					return &duration, nil
				}
			}
		}

		// Fallback: try ErrorInfo.metadata.quotaResetDelay (e.g., "373.801628ms")
		for _, detail := range details.Array() {
			typeVal := detail.Get("@type").String()
			if typeVal == "type.googleapis.com/google.rpc.ErrorInfo" {
				quotaResetDelay := detail.Get("metadata.quotaResetDelay").String()
				if quotaResetDelay != "" {
					duration, err := time.ParseDuration(quotaResetDelay)
					if err == nil {
						return &duration, nil
					}
				}
			}
		}
	}

	// Fallback: parse from error.message "Your quota will reset after Xs."
	message := gjson.GetBytes(errorBody, "error.message").String()
	if message != "" {
		re := regexp.MustCompile(`after\s+(\d+)s\.?`)
		if matches := re.FindStringSubmatch(message); len(matches) > 1 {
			seconds, err := strconv.Atoi(matches[1])
			if err == nil {
				duration := time.Duration(seconds) * time.Second
				return &duration, nil
			}
		}
		reHuman := regexp.MustCompile(`after\s+((?:\d+h)?(?:\d+m)?(?:\d+s)?)\.?`)
		if matches := reHuman.FindStringSubmatch(strings.ToLower(message)); len(matches) > 1 {
			if duration, err := time.ParseDuration(matches[1]); err == nil && duration > 0 {
				return &duration, nil
			}
		}
	}

	return nil, fmt.Errorf("no RetryInfo found")
}
