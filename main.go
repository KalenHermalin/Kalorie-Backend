package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"google.golang.org/genai"
)

func main() {
	fmt.Println("Hello, World")
	mux := http.NewServeMux()
	/* DECLARE ROUTES */
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /analyze/food", analayzeFoodHandler)

	http.ListenAndServe(":8080", mux)
}

func healthHandler(writer http.ResponseWriter, request *http.Request) {
	// Plain Text Response
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	// Json Response
	//writer.Header().Set("Content-Type", "application/json")
	// Wrtiting Status codes
	//writer.WriteHeader(http.StatusBadRequest)
	fmt.Fprint(writer, "Hello World Beyond")

}

type ResponseStatus string

const (
	StatusSuccess ResponseStatus = "success"
	StatusError   ResponseStatus = "error"
)

type Response struct {
	Status  ResponseStatus `json:"status"`
	Message string         `json:"message"`
}

type Request struct {
	Message string `json:"message"`
}

func analayzeFoodHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	/*
		Initalize LLM Connection
	*/
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  "AIzaSyDVoY8-Y3PGpRStwnh_JxnZ46A7R7hL_yU",
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		response := &Response{
			StatusError,
			err.Error(),
		}
		writer.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(writer).Encode(response)
		return

	}
	/*
		Generating Response Via Text Input
	*/
	var requestData Request
	json.NewDecoder(request.Body).Decode(&requestData)
	result, err := client.Models.GenerateContent(context.Background(), "gemini-2.5-flash-lite", genai.Text(requestData.Message), nil)
	if err != nil {
		fmt.Println(err)
		response := &Response{
			StatusError,
			err.Error(),
		}
		writer.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(writer).Encode(response)
		return
	}

	/*
		Writing Response Back
	*/
	writer.WriteHeader(http.StatusOK)
	response := &Response{
		StatusSuccess,
		result.Text(),
	}
	json.NewEncoder(writer).Encode(response)

}
