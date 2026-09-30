// pattern: Functional Core
package intake

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
)

// ExtractVerifiedDirectoryFiles returns the canonical files only after the
// complete archive has passed its path, size, digest, and manifest checks.
func ExtractVerifiedDirectoryFiles(content []byte) ([]DirectoryInputFile, error) {
	verified, err := verifyDirectoryArchive(content)
	if err != nil {
		return nil, err
	}
	var manifest DirectoryManifest
	if err = json.Unmarshal(verified.Manifest, &manifest); err != nil {
		return nil, errors.New("verified directory manifest could not be decoded")
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		return nil, errors.New("verified directory archive could not be reopened")
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(io.LimitReader(gzipReader, MaxDirectoryArchiveExpandedBytes))
	if _, err = tarReader.Next(); err != nil {
		return nil, errors.New("verified directory manifest header is missing")
	}
	files := make([]DirectoryInputFile, 0, len(manifest.Files))
	for _, entry := range manifest.Files {
		header, nextErr := tarReader.Next()
		if nextErr != nil || header.Name != entry.RelativePath || header.Size != entry.ByteSize {
			return nil, errors.New("verified directory file order changed")
		}
		body, readErr := io.ReadAll(io.LimitReader(tarReader, entry.ByteSize+1))
		if readErr != nil || int64(len(body)) != entry.ByteSize {
			return nil, errors.New("verified directory file size changed")
		}
		files = append(files, DirectoryInputFile{RelativePath: entry.RelativePath, MediaType: entry.MediaType, Content: body})
	}
	return files, nil
}
