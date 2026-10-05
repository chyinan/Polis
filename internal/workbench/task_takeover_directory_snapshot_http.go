// pattern: Imperative Shell
package workbench

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"polis/internal/control"
	"polis/internal/core"
	"polis/internal/kernel"
)

var errTaskTakeoverDirectorySnapshotTooLarge = errors.New("Task takeover directory snapshot exceeds its upload limit")

func parseTaskTakeoverDirectorySnapshotUpload(response http.ResponseWriter, request *http.Request) (control.TaskTakeoverDirectorySnapshotCommand, error) {
	request.Body = http.MaxBytesReader(response, request.Body, (7<<20)+(256<<10))
	if err := request.ParseMultipartForm(64 << 10); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return control.TaskTakeoverDirectorySnapshotCommand{}, errTaskTakeoverDirectorySnapshotTooLarge
		}
		return control.TaskTakeoverDirectorySnapshotCommand{}, core.Malformed
	}
	if request.MultipartForm == nil {
		return control.TaskTakeoverDirectorySnapshotCommand{}, core.Malformed
	}
	defer request.MultipartForm.RemoveAll()
	files := request.MultipartForm.File["files"]
	paths := request.MultipartForm.Value["paths"]
	if len(request.MultipartForm.File) != 1 || len(files) == 0 || len(files) > 250 || len(paths) != len(files) {
		return control.TaskTakeoverDirectorySnapshotCommand{}, core.Malformed
	}
	for key, values := range request.MultipartForm.Value {
		if (key != "requestId" && key != "baseWorkspaceTreeSha256" && key != "humanEffortSeconds" && key != "paths") ||
			(key != "paths" && len(values) != 1) {
			return control.TaskTakeoverDirectorySnapshotCommand{}, core.Malformed
		}
	}
	requestIDs := request.MultipartForm.Value["requestId"]
	manifestDigests := request.MultipartForm.Value["baseWorkspaceTreeSha256"]
	effortValues := request.MultipartForm.Value["humanEffortSeconds"]
	if len(requestIDs) != 1 || !core.ValidID(requestIDs[0]) || len(manifestDigests) != 1 || len(manifestDigests[0]) != 64 ||
		len(effortValues) != 1 {
		return control.TaskTakeoverDirectorySnapshotCommand{}, core.Malformed
	}
	if headerRequestID := request.Header.Get("X-Request-ID"); headerRequestID != "" && headerRequestID != requestIDs[0] {
		return control.TaskTakeoverDirectorySnapshotCommand{}, core.Malformed
	}
	effort, err := strconv.ParseInt(effortValues[0], 10, 64)
	if err != nil || effort < 0 || effort > 86400 {
		return control.TaskTakeoverDirectorySnapshotCommand{}, core.Malformed
	}
	result := control.TaskTakeoverDirectorySnapshotCommand{RequestID: requestIDs[0], BaseWorkspaceTreeSHA256: manifestDigests[0], HumanEffortSeconds: effort}
	result.Files = make([]kernel.TaskTakeoverDirectorySnapshotFile, 0, len(files))
	totalBytes := int64(0)
	for index, header := range files {
		if !kernel.ValidWorkspaceRelativePath(paths[index]) {
			return control.TaskTakeoverDirectorySnapshotCommand{}, core.Malformed
		}
		if header.Size <= 0 || header.Size > 2<<20 || totalBytes+header.Size > 7<<20 {
			return control.TaskTakeoverDirectorySnapshotCommand{}, errTaskTakeoverDirectorySnapshotTooLarge
		}
		file, openErr := header.Open()
		if openErr != nil {
			return control.TaskTakeoverDirectorySnapshotCommand{}, core.Malformed
		}
		content, readErr := io.ReadAll(io.LimitReader(file, (2<<20)+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil || len(content) == 0 || int64(len(content)) != header.Size || len(content) > 2<<20 {
			return control.TaskTakeoverDirectorySnapshotCommand{}, core.Malformed
		}
		totalBytes += int64(len(content))
		if totalBytes > 7<<20 {
			return control.TaskTakeoverDirectorySnapshotCommand{}, errTaskTakeoverDirectorySnapshotTooLarge
		}
		result.Files = append(result.Files, kernel.TaskTakeoverDirectorySnapshotFile{RelativePath: paths[index], Content: string(content)})
	}
	return result, nil
}
