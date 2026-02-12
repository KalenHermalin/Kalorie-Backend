package handlers

import "net/http"

type SystemHandler struct {
}

func NewSystemHander() *SystemHandler {
	return &SystemHandler{}
}
func (sh *SystemHandler) HealthHandler(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(http.StatusOK)
	writer.Write([]byte("Ok!"))
}
