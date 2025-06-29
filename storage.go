package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
)

// forwardFilesToStorage forwards uploaded files to the storage server
func (s *Server) forwardFilesToStorage(codebaseID string, files []*multipart.FileHeader, r *http.Request) ([]FileInfo, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add codebase ID
	writer.WriteField("codebase_id", codebaseID)

	var fileInfos []FileInfo

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}
		defer file.Close()

		// Get relative path from form data
		relativePath := r.FormValue("path_" + fileHeader.Filename)
		if relativePath == "" {
			relativePath = fileHeader.Filename
		}

		// Create form file
		part, err := writer.CreateFormFile("files", fileHeader.Filename)
		if err != nil {
			continue
		}

		written, err := io.Copy(part, file)
		if err != nil {
			continue
		}

		// Add path information
		writer.WriteField("path_"+fileHeader.Filename, relativePath)

		fileInfos = append(fileInfos, FileInfo{
			Name: filepath.Base(relativePath),
			Path: relativePath,
			Size: written,
		})
	}

	writer.Close()

	// Send to storage server
	resp, err := http.Post(s.storageServerURL+"/store", writer.FormDataContentType(), &buf)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("storage server returned status %d", resp.StatusCode)
	}

	return fileInfos, nil
}
