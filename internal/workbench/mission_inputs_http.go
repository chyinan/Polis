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

var errMissionInputTooLarge = errors.New("mission input exceeds upload limit")

func parseMissionInputUpload(response http.ResponseWriter, request *http.Request, missionID string) (control.UploadMissionInputRequest, string, string, []byte, error) {
	request.Body = http.MaxBytesReader(response, request.Body, int64(intake.MaxUploadBytes)+(64<<10))
	if err := request.ParseMultipartForm(32 << 10); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return control.UploadMissionInputRequest{}, "", "", nil, errMissionInputTooLarge
		}
		return control.UploadMissionInputRequest{}, "", "", nil, core.Malformed
	}
	if request.MultipartForm == nil {
		return control.UploadMissionInputRequest{}, "", "", nil, core.Malformed
	}
	defer request.MultipartForm.RemoveAll()

	if len(request.MultipartForm.File) != 1 || len(request.MultipartForm.File["file"]) != 1 {
		return control.UploadMissionInputRequest{}, "", "", nil, core.Malformed
	}
	for key, values := range request.MultipartForm.Value {
		if (key != "requestId" && key != "inputId") || len(values) != 1 {
			return control.UploadMissionInputRequest{}, "", "", nil, core.Malformed
		}
	}
	requestIDs := request.MultipartForm.Value["requestId"]
	if len(requestIDs) != 1 {
		return control.UploadMissionInputRequest{}, "", "", nil, core.Malformed
	}
	inputID := ""
	if values := request.MultipartForm.Value["inputId"]; len(values) == 1 {
		inputID = values[0]
	}

	file, header, err := request.FormFile("file")
	if err != nil {
		return control.UploadMissionInputRequest{}, "", "", nil, core.Malformed
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, int64(intake.MaxUploadBytes)+1))
	if err != nil {
		return control.UploadMissionInputRequest{}, "", "", nil, err
	}
	if len(content) > intake.MaxUploadBytes || header.Size > intake.MaxUploadBytes {
		return control.UploadMissionInputRequest{}, "", "", nil, errMissionInputTooLarge
	}

	input := control.UploadMissionInputRequest{MissionID: missionID, InputID: inputID, RequestID: requestIDs[0]}
	return input, header.Filename, header.Header.Get("Content-Type"), content, nil
}
