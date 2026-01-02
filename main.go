package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"google.golang.org/genai"
)

const ANALYZEFOODSYSTEMPROMPT = `You are an advanced AI Nutritionist and Computer Vision Expert. Your sole purpose is to analyze images of food, identify the ingredients and portion sizes with high precision, and estimate the nutritional content.

### INSTRUCTIONS:
1.  **Analyze the Image:** logical scan of the provided image to identify distinct food items.
2.  **Estimate Portions:** Use visual cues (relative size to plates, utensils, or standard portion sizes) to estimate the mass/volume of each item.
3.  **Calculate Macros:** Based on standard nutritional databases, calculate the Calories (kcal), Fat (g), Carbs (g), and Protein (g) for each identified item.
4.  **Generate a Name:** Create a concise, appetizing name for the meal (e.g., 'Grilled Salmon with Quinoa and Asparagus').
5.  **Validation:**
	* If the image clearly contains food, set 'success' to true.
	* If the image is NOT food or is too blurry to analyze, set 'success' to false, and set meal to nil.`

const ANALYZELABELSYSTEMPROMPT = `You are an advanced AI Nutritionist and Computer Vision Expert. Your sole purpose is to analyze images of nutrition labels, identify the serving unit in metric (usually grams or ml, etc), the base quantity for the serving, then the main macro nutrients including calories, fat, carbs, and protein.

### INSTRUCTIONS:
1.  **Locate Label:** Scan the image to identify a "Nutrition Facts" table or standard nutritional information panel.
2.  **Extract Serving Info:** Locate the "Serving Size" line. Parse the text to extract the "base_quantity" (as a number) and the "unit" (strictly metric, e.g., "g", "ml").
    * *Note:* If the label lists "1 cup (228g)", the base quantity is 228 and the unit is "g".
3.  **Extract Macros:** Locate and extract the exact numeric values for **Calories**, **Total Fat**, **Total Carbohydrate**, and **Protein**. Ensure you are extracting the "Total" values, not sub-groups (e.g., do not mistake "Saturated Fat" for "Total Fat").
4.  **Data Formatting:** Ensure "macros" and "base_quantity" are returned as numbers while "unit" remain strings as per the schema.
5.  **Validation:**
    * If a legible nutrition label is detected, set "success" to true.
    * If the image does not contain a nutrition label or is too blurry to read, set "success" to false and provide empty strings/zeros for the data fields.`

var client *genai.Client

const MODEL string = "gemini-2.5-flash-lite"

func main() {
	fmt.Println("Hello, World")
	/*
		Initalize LLM Connection
	*/
	var err error
	client, err = genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  "AIzaSyDVoY8-Y3PGpRStwnh_JxnZ46A7R7hL_yU",
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		fmt.Println(err.Error())
	}
	mux := http.NewServeMux()
	/* DECLARE ROUTES */
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /analyze/food", analayzeFoodHandler)
	mux.HandleFunc("POST /analyze/label", analayzeLabelHandler)

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
	Payload any            `json:"payload"`
}

type Request struct {
	Picture []byte `json:"picture"`
}
type GeminiMealResponse struct {
	Success bool `json:"success"`
	Payload any  `json:"payload"`
}

/*
Same types as gemini response schemas but we dont need them, at least not now
type Meal struct {
	Foodname string `json:"food_name"`
	Cal      uint16 `json:"cal"`
	Fat      uint16 `json:"fat"`
	Carbs    uint16 `json:"carbs"`
	Protein  uint16 `json:"protein"`
}

type Label struct {
	Units        string `json:"units"`
	BaseQuantity uint16 `json:"base_quantity"`
	Macros       Macro  `json:"macros"`
}

type Macro struct {
	Cal     uint16 `json:"cal"`
	Carbs   uint16 `json:"carbs"`
	Fat     uint16 `json:"fat"`
	Protein uint16 `json:"protein"`
}*/

func analayzeFoodHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	/*
		Generating Response Via Text Input
	*/
	var requestData Request
	decodePictureData(request, writer, &requestData)
	userParts := []*genai.Part{

		{InlineData: &genai.Blob{Data: requestData.Picture, MIMEType: "image/jpeg"}},
	}
	systemParts := []*genai.Part{
		{Text: ANALYZEFOODSYSTEMPROMPT},
	}

	result, err := client.Models.GenerateContent(context.Background(), MODEL, []*genai.Content{{Parts: userParts}}, &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{Parts: systemParts},
		ResponseMIMEType:  `application/json`,
		ResponseSchema: &genai.Schema{
			PropertyOrdering: []string{"success", "payload"},
			Type:             genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"success": {Type: genai.TypeBoolean},
				"payload": {Type: genai.TypeObject,
					PropertyOrdering: []string{"food_name", "cal", "fat", "protein", "carbs"},
					Properties: map[string]*genai.Schema{
						"food_name": {Type: genai.TypeString, Description: "Simple name for the meal captured"},
						"cal":       {Type: genai.TypeInteger},
						"fat":       {Type: genai.TypeInteger},
						"protein":   {Type: genai.TypeInteger},
						"carbs":     {Type: genai.TypeInteger},
					},
				},
			},
		},
	})
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
	var geminiResponse GeminiMealResponse
	err = json.Unmarshal([]byte(result.Text()), &geminiResponse)
	if err != nil {
		fmt.Println("AI returned invalid JSON:", err.Error())
		response := &Response{
			StatusError,
			"AI returned invalid JSON Meal Payload",
		}
		json.NewEncoder(writer).Encode(response)
		return
	}
	if geminiResponse.Success == false {
		response := &Response{
			StatusError,
			"No Meal Found In Image",
		}
		json.NewEncoder(writer).Encode(response)
		return
	}

	/*
		Writing Response Back
	*/
	writer.WriteHeader(http.StatusOK)
	response := &Response{
		StatusSuccess,
		geminiResponse.Payload,
	}
	json.NewEncoder(writer).Encode(response)

}

func analayzeLabelHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	/*
		Generating Response Via Text Input
	*/
	var requestData Request
	decodePictureData(request, writer, &requestData)
	userParts := []*genai.Part{

		{InlineData: &genai.Blob{Data: requestData.Picture, MIMEType: "image/jpeg"}},
	}
	systemParts := []*genai.Part{
		{Text: ANALYZELABELSYSTEMPROMPT},
	}

	result, err := client.Models.GenerateContent(context.Background(), MODEL, []*genai.Content{{Parts: userParts}}, &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{Parts: systemParts},
		ResponseMIMEType:  `application/json`,
		ResponseSchema: &genai.Schema{
			Type:             genai.TypeObject,
			PropertyOrdering: []string{"success", "payload"},
			Properties: map[string]*genai.Schema{
				"success": {Type: genai.TypeBoolean},
				"payload": {Type: genai.TypeObject,
					PropertyOrdering: []string{"units", "base_quantity", "macros"},
					Properties: map[string]*genai.Schema{
						"units":         {Type: genai.TypeString},
						"base_quantity": {Type: genai.TypeInteger},
						"macros": {Type: genai.TypeObject,
							PropertyOrdering: []string{"cal", "fat", "protein", "carbs"},
							Properties: map[string]*genai.Schema{
								"cal":     {Type: genai.TypeInteger},
								"fat":     {Type: genai.TypeInteger},
								"protein": {Type: genai.TypeInteger},
								"carbs":   {Type: genai.TypeInteger},
							}},
					},
				},
			},
		},
	})

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
	var geminiResponse GeminiMealResponse
	err = json.Unmarshal([]byte(result.Text()), &geminiResponse)
	if err != nil {
		fmt.Println("AI returned invalid JSON:", err.Error())
		response := &Response{
			StatusError,
			"AI returned invalid JSON Meal Payload",
		}
		json.NewEncoder(writer).Encode(response)
		return
	}
	if geminiResponse.Success == false {
		response := &Response{
			StatusError,
			"No Meal Found In Image",
		}
		json.NewEncoder(writer).Encode(response)
		return
	}

	/*
		Writing Response Back
	*/
	writer.WriteHeader(http.StatusOK)
	response := &Response{
		StatusSuccess,
		geminiResponse.Payload,
	}
	json.NewEncoder(writer).Encode(response)

}

func decodePictureData(request *http.Request, writer http.ResponseWriter, requestData *Request) {

	err := json.NewDecoder(request.Body).Decode(&requestData)
	if err != nil {
		fmt.Println(err)
		response := &Response{
			StatusError,
			err.Error(),
		}
		writer.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(writer).Encode(response)
		return
	}
	if requestData.Picture == nil {
		fmt.Println("Picture Null")
		response := &Response{
			StatusError,
			"Picture Field in Json is Missing",
		}
		writer.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(writer).Encode(response)
		return
	}
	if len(requestData.Picture) == 0 {
		fmt.Println("Picture Null")
		response := &Response{
			StatusError,
			"Picture Data Not Sent",
		}
		writer.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(writer).Encode(response)
		return
	}
}
