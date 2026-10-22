package handlers

import (
	"errors"
	"main/internal/apperrors"
	"main/internal/store"
	"main/internal/utils"
	"net/http"
)

// DocsHandler serves the static marketing/support/privacy pages in
// ./docs (index.html, support.html, privacy.html) - e.g. for the
// Support URL / Privacy Policy URL an App Store / Play Store listing
// requires. The pages' own assets (css/js/image) are served separately
// via a http.FileServer mount in cmd/api.go rather than a handler here,
// since that's just files-as-is with no per-page logic.
type DocsHandler struct {
	docsDir     string
	remindStore store.RemindMeRepo
}

func NewDocsHandler(docsDir string, store store.RemindMeRepo) *DocsHandler {
	return &DocsHandler{docsDir: docsDir, remindStore: store}
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

func (dh *DocsHandler) RobotsTxt(writer http.ResponseWriter, request *http.Request) {
	http.ServeFile(writer, request, dh.docsDir+"/robots.txt")
}

func (dh *DocsHandler) Sitemap(writer http.ResponseWriter, request *http.Request) {
	http.ServeFile(writer, request, dh.docsDir+"/sitemap.xml")
}

type remindMeRequestPayload struct {
	Email string `json:"email"`
}

func (dh *DocsHandler) RemindMe(writer http.ResponseWriter, request *http.Request) error {
	var requestData remindMeRequestPayload
	if err := utils.DecodePayload(request.Body, &requestData); err != nil {
		// Bad Request because all we did was decode it and got an error meaning invalid JSON
		return apperrors.New(apperrors.BadRequest, err)
	}

	err := dh.remindStore.Remind_user(request.Context(), requestData.Email)
	if err != nil {
		if errors.Is(err, store.ErrEmailExists) {
			return apperrors.New(apperrors.Conflict, err)
		}
		return err
	}

	return nil
}
