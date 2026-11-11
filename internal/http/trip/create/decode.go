package create

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/gmlazutin/avito-go-template/internal/api/httpv1/gen"
)

const maxRequestBodySize = 1 << 20

func decodeRequest(w http.ResponseWriter, r *http.Request) (httpv1gen.CreateTripJSONRequestBody, error) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return httpv1gen.CreateTripJSONRequestBody{}, errors.New("Content-Type must be application/json")
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return httpv1gen.CreateTripJSONRequestBody{}, errors.New("request body is invalid or exceeds size limit")
	}
	if err := requireFields(body); err != nil {
		return httpv1gen.CreateTripJSONRequestBody{}, err
	}

	var request httpv1gen.CreateTripJSONRequestBody
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return httpv1gen.CreateTripJSONRequestBody{}, errors.New("request body is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return httpv1gen.CreateTripJSONRequestBody{}, errors.New("request body must contain a single JSON object")
	}
	return request, nil
}

func requireFields(body []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return errors.New("request body is invalid")
	}
	for _, name := range []string{"user_id", "driver_id", "start_point", "end_point", "price"} {
		if missingJSONValue(fields[name]) {
			return errors.New("request body is missing required fields")
		}
	}
	for _, name := range []string{"start_point", "end_point"} {
		var coordinates map[string]json.RawMessage
		if err := json.Unmarshal(fields[name], &coordinates); err != nil {
			return errors.New("request coordinates are invalid")
		}
		if missingJSONValue(coordinates["latitude"]) || missingJSONValue(coordinates["longitude"]) {
			return errors.New("request coordinates are missing required fields")
		}
	}
	return nil
}

func missingJSONValue(value json.RawMessage) bool {
	return len(value) == 0 || bytes.Equal(bytes.TrimSpace(value), []byte("null"))
}
