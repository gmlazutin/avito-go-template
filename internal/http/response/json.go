package response

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func JSON(w http.ResponseWriter, status int, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	payload = append(payload, '\n')
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("write response: %w", err)
	}
	return nil
}
