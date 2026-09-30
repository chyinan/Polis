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

var errMissionDirectoryInputTooLarge = errors.New("mission directory input exceeds upload limit")

func parseMissionDirectoryUpload(response http.ResponseWriter, request *http.Request, missionID string) (control.UploadMissionInputRequest, []intake.DirectoryInputFile, error) {
	request.Body = http.MaxBytesReader(response, request.Body, int64(intake.MaxDirectoryBytes)+(256<<10))
	if err := request.ParseMultipartForm(64 << 10); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return control.UploadMissionInputRequest{}, nil, errMissionDirectoryInputTooLarge
		}
		return control.UploadMissionInputRequest{}, nil, core.Malformed
	}
	if request.MultipartForm == nil {
		return control.UploadMissionInputRequest{}, nil, core.Malformed
	}
	defer request.MultipartForm.RemoveAll()
	if len(request.MultipartForm.File) != 1 || len(request.MultipartForm.File["files"]) == 0 || len(request.MultipartForm.File["files"]) > intake.MaxDirectoryFiles {
		return control.UploadMissionInputRequest{}, nil, core.Malformed
	}
	for key, values := range request.MultipartForm.Value {
		if (key != "requestId" && key != "inputId" && key != "paths") || (key != "paths" && len(values) != 1) {
			return control.UploadMissionInputRequest{}, nil, core.Malformed
		}
	}
	requestIDs := request.MultipartForm.Value["requestId"]
	paths := request.MultipartForm.Value["paths"]
	if len(requestIDs) != 1 || len(paths) != len(request.MultipartForm.File["files"]) {
		return control.UploadMissionInputRequest{}, nil, core.Malformed
	}
	inputID := ""
	if values := request.MultipartForm.Value["inputId"]; len(values) == 1 {
		inputID = values[0]
	}
	files := request.MultipartForm.File["files"]
	inputs := make([]intake.DirectoryInputFile, 0, len(files))
	totalBytes := int64(0)
	for index, header := range files {
		if header.Size <= 0 || header.Size > intake.MaxUploadBytes || totalBytes+header.Size > intake.MaxDirectoryBytes {
			return control.UploadMissionInputRequest{}, nil, errMissionDirectoryInputTooLarge
		}
		file, err := header.Open()
		if err != nil {
			return control.UploadMissionInputRequest{}, nil, core.Malformed
		}
		content, readErr := io.ReadAll(io.LimitReader(file, intake.MaxUploadBytes+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return control.UploadMissionInputRequest{}, nil, core.Malformed
		}
		if len(content) == 0 || int64(len(content)) != header.Size || len(content) > intake.MaxUploadBytes {
			return control.UploadMissionInputRequest{}, nil, core.Malformed
		}
		totalBytes += int64(len(content))
		if totalBytes > intake.MaxDirectoryBytes {
			return control.UploadMissionInputRequest{}, nil, errMissionDirectoryInputTooLarge
		}
		inputs = append(inputs, intake.DirectoryInputFile{RelativePath: paths[index], MediaType: header.Header.Get("Content-Type"), Content: content})
	}
	return control.UploadMissionInputRequest{MissionID: missionID, InputID: inputID, RequestID: requestIDs[0]}, inputs, nil
}
