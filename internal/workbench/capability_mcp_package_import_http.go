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

var errStdioMCPPackageTooLarge = errors.New("stdio MCP package ZIP exceeds upload limit")

func parseStdioMCPPackageUpload(response http.ResponseWriter, request *http.Request) (control.ImportStdioMCPPackageRequest, error) {
	request.Body = http.MaxBytesReader(response, request.Body, int64(intake.MaxUploadBytes)+(64<<10))
	if err := request.ParseMultipartForm(32 << 10); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return control.ImportStdioMCPPackageRequest{}, errStdioMCPPackageTooLarge
		}
		return control.ImportStdioMCPPackageRequest{}, core.Malformed
	}
	if request.MultipartForm == nil {
		return control.ImportStdioMCPPackageRequest{}, core.Malformed
	}
	defer request.MultipartForm.RemoveAll()
	if len(request.MultipartForm.File) != 1 || len(request.MultipartForm.File["bundle"]) != 1 {
		return control.ImportStdioMCPPackageRequest{}, core.Malformed
	}
	for key, values := range request.MultipartForm.Value {
		if (key != "serverId" && key != "revision" && key != "requestId") || len(values) != 1 {
			return control.ImportStdioMCPPackageRequest{}, core.Malformed
		}
	}
	serverIDs := request.MultipartForm.Value["serverId"]
	revisions := request.MultipartForm.Value["revision"]
	requestIDs := request.MultipartForm.Value["requestId"]
	if len(serverIDs) > 1 || len(revisions) != 1 || len(requestIDs) != 1 || (len(serverIDs) == 1 && len(serverIDs[0]) > 80) || len(revisions[0]) > 64 || len(requestIDs[0]) > 80 {
		return control.ImportStdioMCPPackageRequest{}, core.Malformed
	}
	file, header, err := request.FormFile("bundle")
	if err != nil {
		return control.ImportStdioMCPPackageRequest{}, core.Malformed
	}
	defer file.Close()
	archive, err := io.ReadAll(io.LimitReader(file, int64(intake.MaxUploadBytes)+1))
	if err != nil || len(archive) == 0 {
		return control.ImportStdioMCPPackageRequest{}, core.Malformed
	}
	if len(archive) > intake.MaxUploadBytes || header.Size > intake.MaxUploadBytes {
		return control.ImportStdioMCPPackageRequest{}, errStdioMCPPackageTooLarge
	}
	serverID := ""
	if len(serverIDs) == 1 {
		serverID = serverIDs[0]
	}
	return control.ImportStdioMCPPackageRequest{ServerID: serverID, Revision: revisions[0], RequestID: requestIDs[0], Archive: archive}, nil
}
