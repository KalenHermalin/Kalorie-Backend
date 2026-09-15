"""Port of internal/utils/utils.go"""

from __future__ import annotations

import base64
import os
from typing import Type, TypeVar

from pydantic import BaseModel, ValidationError

T = TypeVar("T", bound=BaseModel)


def decode_payload(body: bytes, model_cls: Type[T]) -> T:
    """Mirrors utils.DecodePayload: decode a JSON body into `model_cls`.

    Raises ValueError (mirroring Go's decode error) if the body isn't valid
    JSON for the given model.
    """
    try:
        return model_cls.model_validate_json(body)
    except ValidationError as e:
        raise ValueError(str(e)) from e


def generate_secure_string(n: int) -> str:
    b = os.urandom(n)
    # base64 encoding makes it URL-safe for your Refresh Tokens
    return base64.urlsafe_b64encode(b).decode("ascii")


ANALYZEFOODSYSTEMPROMPT = """You are an advanced AI Nutritionist and Computer Vision Expert. Your sole purpose is to analyze images of food, identify the ingredients and portion sizes with high precision, and estimate the nutritional content.

### INSTRUCTIONS:
1.  **Analyze the Image:** logical scan of the provided image to identify distinct food items.
2.  **Estimate Portions:** Use visual cues (relative size to plates, utensils, or standard portion sizes) to estimate the mass/volume of each item.
3.  **Calculate Macros:** Based on standard nutritional databases, calculate the Calories (kcal), Fat (g), Carbs (g), and Protein (g) for each identified item.
4.  **Generate a Name:** Create a concise, appetizing name for the meal (e.g., 'Grilled Salmon with Quinoa and Asparagus').
5.  **Validation:**
	* If the image clearly contains food, set 'success' to true.
	* If the image is NOT food or is too blurry to analyze, set 'success' to false, and set meal to nil."""

ANALYZELABELSYSTEMPROMPT = """You are an advanced AI Nutritionist and Computer Vision Expert. Your sole purpose is to analyze images of nutrition labels, identify the serving unit in metric (usually grams or ml, etc), the base quantity for the serving, then the main macro nutrients including calories, fat, carbs, and protein.

### INSTRUCTIONS:
1.  **Locate Label:** Scan the image to identify a "Nutrition Facts" table or standard nutritional information panel.
2.  **Extract Serving Info:** Locate the "Serving Size" line. Parse the text to extract the "base_quantity" (as a number) and the "unit" (strictly metric, e.g., "g", "ml").
    * *Note:* If the label lists "1 cup (228g)", the base quantity is 228 and the unit is "g".
3.  **Extract Macros:** Locate and extract the exact numeric values for **Calories**, **Total Fat**, **Total Carbohydrate**, and **Protein**. Ensure you are extracting the "Total" values, not sub-groups (e.g., do not mistake "Saturated Fat" for "Total Fat").
4.  **Data Formatting:** Ensure "macros" and "base_quantity" are returned as numbers while "unit" remain strings as per the schema.
5.  **Validation:**
    * If a legible nutrition label is detected, set "success" to true.
    * If the image does not contain a nutrition label or is too blurry to read, set "success" to false and provide empty strings/zeros for the data fields."""


def check_valid_string(s: str) -> None:
    if len(s.strip()) <= 0:
        raise ValueError("Code is Empty")
