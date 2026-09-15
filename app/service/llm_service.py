"""Port of internal/service/llm.go"""

from app.models.llm import LLMProvider


class LLMService:
    def __init__(self, provider: LLMProvider):
        self.provider = provider


# TODO: Create default funciton which then calls the provider
# Single analyze photo function which will work for meals and labels?
