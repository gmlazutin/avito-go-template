package middleware

import (
	"fmt"
	"net/http"

	"github.com/gmlazutin/avito-go-template/internal/http/domainerror"
)

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err := fmt.Errorf("panic recovered: %v", recovered)
				recorder := recorderFrom(w)
				if recorder != nil {
					_, _, wroteHeader, _ := recorder.snapshot()
					if wroteHeader {
						return
					}
				}
				domainerror.Write(w, r, err)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
