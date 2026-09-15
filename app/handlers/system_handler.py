"""Port of internal/handlers/system.go"""

from fastapi import Response


class SystemHandler:
    def health_handler(self) -> Response:
        return Response(content="Ok!", status_code=200)
