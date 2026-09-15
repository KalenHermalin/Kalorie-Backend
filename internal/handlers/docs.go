package handlers

import "net/http"

// DocsHandler serves the static marketing/support/privacy pages in
// ./docs (index.html, support.html, privacy.html) - e.g. for the
// Support URL / Privacy Policy URL an App Store / Play Store listing
// requires. The pages' own assets (css/js/image) are served separately
// via a http.FileServer mount in cmd/api.go rather than a handler here,
// since that's just files-as-is with no per-page logic.
type DocsHandler struct {
	docsDir string
}

func NewDocsHandler(docsDir string) *DocsHandler {
	return &DocsHandler{docsDir: docsDir}
}

func (dh *DocsHandler) IndexPage(writer http.ResponseWriter, request *http.Request) {
	http.ServeFile(writer, request, dh.docsDir+"/index.html")
}

func (dh *DocsHandler) SupportPage(writer http.ResponseWriter, request *http.Request) {
	http.ServeFile(writer, request, dh.docsDir+"/support.html")
}

func (dh *DocsHandler) PrivacyPage(writer http.ResponseWriter, request *http.Request) {
	http.ServeFile(writer, request, dh.docsDir+"/privacy.html")
}
