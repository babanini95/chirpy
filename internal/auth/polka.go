package auth

import (
	"fmt"
	"net/http"
	"strings"
)

func GetAPIKey(headers http.Header) (string, error) {
	if headers == nil {
		return "", fmt.Errorf("no headers found")
	}

	apiKey := strings.TrimPrefix(headers.Get("Authorization"), "ApiKey ")
	if apiKey == "" {
		return "", fmt.Errorf("token doesn't exist")
	}

	return apiKey, nil
}
