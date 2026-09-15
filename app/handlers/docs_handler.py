"""Serves the static marketing/support/privacy pages that live in
app/docs (index.html, support.html, privacy.html) - e.g. for the
Support URL / Privacy Policy URL an App Store / Play Store listing
needs. The pages' own CSS/JS/image assets under app/docs/assets are
served separately via a StaticFiles mount in api.py rather than a
hand-written handler here, since that's just files-as-is with no
per-page logic - no reason to hand-roll path/MIME-type handling
StaticFiles already does correctly.
"""

from __future__ import annotations

from pathlib import Path

from fastapi.responses import FileResponse

DOCS_DIR = Path(__file__).resolve().parent.parent / "docs"


class DocsHandler:
    def index_page(self) -> FileResponse:
        return FileResponse(DOCS_DIR / "index.html", media_type="text/html")

    def support_page(self) -> FileResponse:
        return FileResponse(DOCS_DIR / "support.html", media_type="text/html")

    def privacy_page(self) -> FileResponse:
        return FileResponse(DOCS_DIR / "privacy.html", media_type="text/html")
