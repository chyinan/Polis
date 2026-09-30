// pattern: Imperative Shell
package workbench

import (
	"errors"
	"io"
	"net/http"

	"polis/internal/control"
	"polis/internal/core"
	"polis/internal/intake"
)

var errSkillPackageTooLarge = errors.New("Skill source ZIP exceeds upload limit")

func parseReadOnlySkillPackageUpload(response http.ResponseWriter, request *http.Request) (control.ImportReadOnlySkillPackageRequest, error) {
	request.Body = http.MaxBytesReader(response, request.Body, int64(intake.MaxUploadBytes)+(64<<10))
	if err := request.ParseMultipartForm(32 << 10); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return control.ImportReadOnlySkillPackageRequest{}, errSkillPackageTooLarge
		}
		return control.ImportReadOnlySkillPackageRequest{}, core.Malformed
	}
	if request.MultipartForm == nil {
		return control.ImportReadOnlySkillPackageRequest{}, core.Malformed
	}
	defer request.MultipartForm.RemoveAll()
	if len(request.MultipartForm.File) != 1 || len(request.MultipartForm.File["bundle"]) != 1 {
		return control.ImportReadOnlySkillPackageRequest{}, core.Malformed
	}
	for key, values := range request.MultipartForm.Value {
		if (key != "revision" && key != "requestId") || len(values) != 1 {
			return control.ImportReadOnlySkillPackageRequest{}, core.Malformed
		}
	}
	revisions := request.MultipartForm.Value["revision"]
	requestIDs := request.MultipartForm.Value["requestId"]
	if len(revisions) != 1 || len(requestIDs) != 1 || len(revisions[0]) > 64 || len(requestIDs[0]) > 80 {
		return control.ImportReadOnlySkillPackageRequest{}, core.Malformed
	}
	file, header, err := request.FormFile("bundle")
	if err != nil {
		return control.ImportReadOnlySkillPackageRequest{}, core.Malformed
	}
	defer file.Close()
	archive, err := io.ReadAll(io.LimitReader(file, int64(intake.MaxUploadBytes)+1))
	if err != nil {
		return control.ImportReadOnlySkillPackageRequest{}, core.Malformed
	}
	if len(archive) == 0 {
		return control.ImportReadOnlySkillPackageRequest{}, core.Malformed
	}
	if len(archive) > intake.MaxUploadBytes || header.Size > intake.MaxUploadBytes {
		return control.ImportReadOnlySkillPackageRequest{}, errSkillPackageTooLarge
	}
	return control.ImportReadOnlySkillPackageRequest{Revision: revisions[0], RequestID: requestIDs[0], Archive: archive}, nil
}
