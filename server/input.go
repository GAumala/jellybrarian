package server

import (
	"fmt"
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
