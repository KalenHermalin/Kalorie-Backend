package llm

import "errors"

var NoFoodFound = errors.New("no food found in provided image")
var ProviderError = errors.New("there is an issue with the current llm provider")
var NoLabelFound = errors.New("no label found in provided image")
