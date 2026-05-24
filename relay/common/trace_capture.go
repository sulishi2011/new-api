package common

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	common2 "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/tracestore"
)

const LogTraceInlineLimit = 64 << 10

type TracePayloadPart struct {
	Headers               map[string][]string `json:"headers,omitempty"`
	Body                  string              `json:"body,omitempty"`
	BodySize              int64               `json:"body_size,omitempty"`
	ContentType           string              `json:"content_type,omitempty"`
	Truncated             bool                `json:"truncated,omitempty"`
	StorageKind           string              `json:"storage_kind,omitempty"`
	BodyObjectKey         string              `json:"body_object_key,omitempty"`
	BodyObjectSize        int64               `json:"body_object_size,omitempty"`
	FullBodyTruncated     bool                `json:"full_body_truncated,omitempty"`
	FullBodyError         string              `json:"full_body_error,omitempty"`
	bodyFilePath          string
	bodyFile              *os.File
	bodyWriter            *bufio.Writer
	bodyFileClosed        bool
	bodyFileHandedOff     bool
	bodyObjectContentType string
	bodyMu                sync.Mutex
}

type TracePayload struct {
	Version           int               `json:"version"`
	Request           *TracePayloadPart `json:"request,omitempty"`
	Response          *TracePayloadPart `json:"response,omitempty"`
	UpstreamRequestId string            `json:"upstream_request_id,omitempty"`
	StatusCode        int               `json:"status_code,omitempty"`
}

type TraceFullBodyFile struct {
	Kind        string
	Path        string
	ObjectKey   string
	ContentType string
	Size        int64
}

func (info *RelayInfo) TakeTracePayload() *TracePayload {
	if info == nil {
		return nil
	}
	payload := info.TracePayload
	info.TracePayload = nil
	return payload
}

func (info *RelayInfo) DiscardTracePayload() {
	payload := info.TakeTracePayload()
	if payload == nil {
		return
	}
	payload.CleanupFullBodyFiles()
}

func (info *RelayInfo) ensureTracePayload() *TracePayload {
	if info == nil {
		return nil
	}
	if info.TracePayload == nil {
		info.TracePayload = &TracePayload{
			Version: 1,
		}
	}
	return info.TracePayload
}

func cloneHeader(header http.Header) map[string][]string {
	if len(header) == 0 {
		return nil
	}
	cloned := make(map[string][]string, len(header))
	for key, values := range header {
		if len(values) == 0 {
			continue
		}
		items := make([]string, len(values))
		copy(items, values)
		cloned[key] = items
	}
	return cloned
}

func shouldInlineTraceBody(contentType string, body []byte) bool {
	lowerContentType := strings.ToLower(strings.TrimSpace(contentType))
	if lowerContentType == "" {
		return utf8.Valid(body)
	}
	if strings.Contains(lowerContentType, "multipart/form-data") {
		return false
	}
	if strings.HasPrefix(lowerContentType, "image/") ||
		strings.HasPrefix(lowerContentType, "audio/") ||
		strings.HasPrefix(lowerContentType, "video/") ||
		strings.Contains(lowerContentType, "application/octet-stream") {
		return false
	}
	if strings.HasPrefix(lowerContentType, "text/") ||
		strings.Contains(lowerContentType, "json") ||
		strings.Contains(lowerContentType, "xml") ||
		strings.Contains(lowerContentType, "javascript") ||
		strings.Contains(lowerContentType, "x-www-form-urlencoded") ||
		strings.Contains(lowerContentType, "graphql") {
		return true
	}
	return utf8.Valid(body)
}

func inferTraceStorageKind(contentType string, body []byte) string {
	lowerContentType := strings.ToLower(strings.TrimSpace(contentType))
	if len(body) == 0 {
		return "empty"
	}
	if strings.Contains(lowerContentType, "multipart/form-data") {
		return "omitted_multipart"
	}
	if !shouldInlineTraceBody(contentType, body) {
		return "omitted_binary"
	}
	return "inline_text"
}

func fillTracePart(part *TracePayloadPart, contentType string, preview []byte, bodySize int64, truncated bool) {
	if part == nil {
		return
	}
	part.ContentType = contentType
	part.BodySize = bodySize
	part.Truncated = truncated
	part.StorageKind = inferTraceStorageKind(contentType, preview)
	if part.StorageKind != "inline_text" {
		part.Body = ""
		return
	}
	part.Body = string(preview)
}

func traceFullBodyEnabled() bool {
	if strings.ToLower(strings.TrimSpace(common2.TraceCaptureMode)) != "all" {
		return false
	}
	return common2.TraceStorageEnabled && common2.TraceFullBodyEnabled && tracestore.IsConfigured()
}

func (part *TracePayloadPart) ensureFullBodyFileLocked(contentType string) bool {
	if !traceFullBodyEnabled() || part == nil || part.FullBodyTruncated || part.FullBodyError != "" || part.bodyFileHandedOff {
		return false
	}
	if part.bodyFile != nil {
		if contentType != "" && part.bodyObjectContentType == "" {
			part.bodyObjectContentType = contentType
		}
		return true
	}
	filePath, file, err := common2.CreateDiskCacheFile(common2.DiskCacheTypeTrace)
	if err != nil {
		part.FullBodyError = err.Error()
		return false
	}
	part.bodyFilePath = filePath
	part.bodyFile = file
	part.bodyWriter = bufio.NewWriterSize(file, 32<<10)
	part.bodyObjectContentType = contentType
	return true
}

func (part *TracePayloadPart) writeFullBodyBytes(contentType string, body []byte) {
	if part == nil || len(body) == 0 {
		return
	}
	part.bodyMu.Lock()
	defer part.bodyMu.Unlock()

	if !part.ensureFullBodyFileLocked(contentType) {
		return
	}
	writeBody := body
	maxBytes := common2.TraceFullBodyMaxBytes
	if maxBytes > 0 {
		remaining := maxBytes - part.BodyObjectSize
		if remaining <= 0 {
			part.FullBodyTruncated = true
			return
		}
		if int64(len(writeBody)) > remaining {
			writeBody = writeBody[:remaining]
			part.FullBodyTruncated = true
		}
	}
	if len(writeBody) == 0 {
		return
	}
	if _, err := part.bodyWriter.Write(writeBody); err != nil {
		part.FullBodyError = err.Error()
		return
	}
	part.BodyObjectSize += int64(len(writeBody))
}

func (part *TracePayloadPart) writeFullBodyString(contentType string, body string) {
	if part == nil || body == "" {
		return
	}
	part.bodyMu.Lock()
	defer part.bodyMu.Unlock()

	if !part.ensureFullBodyFileLocked(contentType) {
		return
	}
	writeBody := body
	maxBytes := common2.TraceFullBodyMaxBytes
	if maxBytes > 0 {
		remaining := maxBytes - part.BodyObjectSize
		if remaining <= 0 {
			part.FullBodyTruncated = true
			return
		}
		if int64(len(writeBody)) > remaining {
			writeBody = writeBody[:remaining]
			part.FullBodyTruncated = true
		}
	}
	if writeBody == "" {
		return
	}
	if _, err := part.bodyWriter.WriteString(writeBody); err != nil {
		part.FullBodyError = err.Error()
		return
	}
	part.BodyObjectSize += int64(len(writeBody))
}

func (part *TracePayloadPart) closeFullBodyFileLocked() error {
	if part == nil || part.bodyFile == nil || part.bodyFileClosed {
		return nil
	}
	if part.bodyWriter != nil {
		if err := part.bodyWriter.Flush(); err != nil {
			part.FullBodyError = err.Error()
			_ = part.bodyFile.Close()
			part.bodyFileClosed = true
			return err
		}
	}
	if err := part.bodyFile.Close(); err != nil {
		part.FullBodyError = err.Error()
		part.bodyFileClosed = true
		return err
	}
	part.bodyFileClosed = true
	return nil
}

func (part *TracePayloadPart) prepareFullBodyFile(kind string, objectKey string) (*TraceFullBodyFile, error) {
	if part == nil {
		return nil, nil
	}
	part.bodyMu.Lock()
	defer part.bodyMu.Unlock()
	if part.bodyFileHandedOff || part.bodyFilePath == "" || part.BodyObjectSize <= 0 {
		return nil, nil
	}
	if err := part.closeFullBodyFileLocked(); err != nil {
		return nil, err
	}
	filePath := part.bodyFilePath
	part.bodyFilePath = ""
	part.bodyFile = nil
	part.bodyWriter = nil
	part.bodyFileClosed = true
	part.bodyFileHandedOff = true
	part.BodyObjectKey = objectKey
	contentType := part.bodyObjectContentType
	if contentType == "" {
		contentType = part.ContentType
	}
	return &TraceFullBodyFile{
		Kind:        kind,
		Path:        filePath,
		ObjectKey:   objectKey,
		ContentType: contentType,
		Size:        part.BodyObjectSize,
	}, nil
}

func (part *TracePayloadPart) cleanupFullBodyFile() {
	if part == nil {
		return
	}
	part.bodyMu.Lock()
	defer part.bodyMu.Unlock()
	_ = part.closeFullBodyFileLocked()
	if part.bodyFilePath != "" {
		_ = os.Remove(part.bodyFilePath)
		part.bodyFilePath = ""
	}
}

func (payload *TracePayload) PrepareFullBodyFiles(requestObjectKey string, responseObjectKey string) ([]TraceFullBodyFile, error) {
	if payload == nil {
		return nil, nil
	}
	files := make([]TraceFullBodyFile, 0, 2)
	if file, err := payload.Request.prepareFullBodyFile("request", requestObjectKey); err != nil {
		return nil, err
	} else if file != nil {
		files = append(files, *file)
	}
	if file, err := payload.Response.prepareFullBodyFile("response", responseObjectKey); err != nil {
		for _, prepared := range files {
			if prepared.Path != "" {
				_ = os.Remove(prepared.Path)
			}
		}
		return nil, err
	} else if file != nil {
		files = append(files, *file)
	}
	return files, nil
}

func (payload *TracePayload) CleanupFullBodyFiles() {
	if payload == nil {
		return
	}
	payload.Request.cleanupFullBodyFile()
	payload.Response.cleanupFullBodyFile()
}

func (info *RelayInfo) SetTraceRequestHeaders(header http.Header) {
	payload := info.ensureTracePayload()
	if payload == nil {
		return
	}
	if payload.Request == nil {
		payload.Request = &TracePayloadPart{}
	}
	payload.Request.Headers = cloneHeader(header)
}

func (info *RelayInfo) SetTraceResponseHeaders(resp *http.Response) {
	payload := info.ensureTracePayload()
	if payload == nil || resp == nil {
		return
	}
	if payload.Response == nil {
		payload.Response = &TracePayloadPart{}
	}
	payload.Response.Headers = cloneHeader(resp.Header)
	payload.Response.ContentType = resp.Header.Get("Content-Type")
	payload.StatusCode = resp.StatusCode
	if upstreamRequestId := extractUpstreamRequestId(resp.Header); upstreamRequestId != "" {
		payload.UpstreamRequestId = upstreamRequestId
	}
}

func extractUpstreamRequestId(header http.Header) string {
	for _, key := range []string{
		"x-request-id",
		"request-id",
		"anthropic-request-id",
		"x-openai-request-id",
		"openai-request-id",
		"x-b3-traceid",
	} {
		if value := strings.TrimSpace(header.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func (info *RelayInfo) SetTraceRequestBodyPreview(contentType string, preview []byte, bodySize int64, truncated bool) {
	payload := info.ensureTracePayload()
	if payload == nil {
		return
	}
	if payload.Request == nil {
		payload.Request = &TracePayloadPart{}
	}
	fillTracePart(payload.Request, contentType, preview, bodySize, truncated)
}

func (info *RelayInfo) SetTraceRequestFullBodyFromBytes(contentType string, body []byte) {
	payload := info.ensureTracePayload()
	if payload == nil || len(body) == 0 {
		return
	}
	if payload.Request == nil {
		payload.Request = &TracePayloadPart{}
	}
	payload.Request.writeFullBodyBytes(contentType, body)
}

func (info *RelayInfo) SetTraceRequestFullBodyFromReader(contentType string, reader io.ReadSeeker) {
	payload := info.ensureTracePayload()
	if payload == nil || reader == nil {
		return
	}
	if payload.Request == nil {
		payload.Request = &TracePayloadPart{}
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		payload.Request.FullBodyError = err.Error()
		return
	}
	buf := make([]byte, 32<<10)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			payload.Request.writeFullBodyBytes(contentType, buf[:n])
			if payload.Request.FullBodyTruncated {
				break
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			payload.Request.FullBodyError = err.Error()
			break
		}
	}
	_, _ = reader.Seek(0, io.SeekStart)
}

func (info *RelayInfo) SetTraceResponseBodyPreview(contentType string, preview []byte, bodySize int64, truncated bool) {
	payload := info.ensureTracePayload()
	if payload == nil {
		return
	}
	if payload.Response == nil {
		payload.Response = &TracePayloadPart{}
	}
	fillTracePart(payload.Response, contentType, preview, bodySize, truncated)
}

func (info *RelayInfo) SetTraceResponseFullBodyFromBytes(contentType string, body []byte) {
	payload := info.ensureTracePayload()
	if payload == nil || len(body) == 0 {
		return
	}
	if payload.Response == nil {
		payload.Response = &TracePayloadPart{}
	}
	payload.Response.writeFullBodyBytes(contentType, body)
}

func (info *RelayInfo) AppendTraceResponseChunk(chunk string) {
	info.AppendTraceResponseChunkParts(chunk)
}

func (info *RelayInfo) AppendTraceResponseChunkParts(chunks ...string) {
	payload := info.ensureTracePayload()
	if payload == nil || len(chunks) == 0 {
		return
	}
	if payload.Response == nil {
		payload.Response = &TracePayloadPart{
			StorageKind: "inline_text",
		}
	}
	part := payload.Response
	if part.StorageKind == "" {
		part.StorageKind = "inline_text"
	}
	if part.ContentType == "" {
		part.ContentType = "text/event-stream"
	}
	for _, chunk := range chunks {
		if chunk == "" {
			continue
		}
		part.BodySize += int64(len(chunk))
		part.writeFullBodyString(part.ContentType, chunk)
	}
	if part.StorageKind != "inline_text" {
		return
	}
	if part.Truncated || len(part.Body) >= LogTraceInlineLimit {
		part.Truncated = true
		return
	}
	for _, chunk := range chunks {
		if chunk == "" {
			continue
		}
		remain := LogTraceInlineLimit - len(part.Body)
		if remain <= 0 {
			part.Truncated = true
			return
		}
		if len(chunk) > remain {
			part.Body += chunk[:remain]
			part.Truncated = true
			return
		}
		part.Body += chunk
	}
}
