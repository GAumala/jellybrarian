package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

func queryInt(r *http.Request, key string, defaultVal int) (int, error) {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return n, nil
}

type filesBody struct {
	Files map[string]string `json:"files"`
}

func parseFilesBody(r *http.Request) map[string]string {
	if r.Body == nil {
		return nil
	}
	var body filesBody
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil
	}
	return body.Files
}
