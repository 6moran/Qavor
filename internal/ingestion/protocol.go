package ingestion

import (
	"errors"
	"fmt"
)

const parserProtocolVersion = 1

var (
	// ErrPythonWorkerCrashed means the local parser process or one of its pipes ended.
	ErrPythonWorkerCrashed = errors.New("python parser worker crashed")
	// ErrPythonProtocol means the parser returned a message outside the JSONL contract.
	ErrPythonProtocol = errors.New("python parser protocol error")
)

type parserRequest struct {
	Type        string `json:"type"`
	Version     int    `json:"version"`
	RequestID   string `json:"request_id"`
	InputPath   string `json:"input_path"`
	Filename    string `json:"filename"`
	OCREngine   string `json:"ocr_engine"`
	OCRAPIURL   string `json:"ocr_api_url,omitempty"`
	OCRAPIKey   string `json:"ocr_api_key,omitempty"`
	OCRAPIModel string `json:"ocr_api_model,omitempty"`
}

type parserReady struct {
	Type         string `json:"type"`
	Version      int    `json:"version"`
	Capabilities struct {
		Docling  bool `json:"docling"`
		RapidOCR bool `json:"rapidocr"`
		APIOCR   bool `json:"api_ocr"`
	} `json:"capabilities"`
}

type parserResponse struct {
	Type      string       `json:"type"`
	Version   int          `json:"version"`
	RequestID string       `json:"request_id"`
	OK        bool         `json:"ok"`
	Result    *ParseResult `json:"result,omitempty"`
	Error     *parserError `json:"error,omitempty"`
}

type parserError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func validateParserReady(ready parserReady) error {
	if ready.Type != "ready" || ready.Version != parserProtocolVersion {
		return fmt.Errorf("%w: invalid ready message", ErrPythonProtocol)
	}
	return nil
}

func validateParserResponse(response parserResponse, requestID string) error {
	if response.Type != "result" || response.Version != parserProtocolVersion {
		return fmt.Errorf("%w: invalid result envelope", ErrPythonProtocol)
	}
	if response.RequestID != requestID || response.RequestID == "" {
		return fmt.Errorf("%w: request id mismatch", ErrPythonProtocol)
	}
	if response.OK {
		if response.Result == nil {
			return fmt.Errorf("%w: successful result is missing", ErrPythonProtocol)
		}
		return nil
	}
	if response.Error == nil || response.Error.Code == "" || response.Error.Message == "" {
		return fmt.Errorf("%w: parser error is incomplete", ErrPythonProtocol)
	}
	return nil
}
