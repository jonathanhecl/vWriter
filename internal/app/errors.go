package app

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jonathanhecl/vWriter/internal/engine"
	"github.com/jonathanhecl/vWriter/internal/media"
	"github.com/jonathanhecl/vWriter/internal/ollama"
	"github.com/jonathanhecl/vWriter/internal/prompt"
)

// errorText renders the user-facing message of any app error.
func errorText(err error) string {
	if coded := codedError(err); coded != nil {
		return coded.Message
	}
	return err.Error()
}

// errorDetails renders the optional structured details as indented JSON.
func errorDetails(err error) string {
	if coded := codedError(err); coded != nil && coded.Details != nil {
		raw, marshalErr := json.MarshalIndent(coded.Details, "", "  ")
		if marshalErr == nil {
			return string(raw)
		}
		return fmt.Sprint(coded.Details)
	}
	return ""
}

// codedError unwraps any of the typed app errors.
func codedError(err error) *struct {
	Code, Message string
	Details       any
} {
	var oerr *ollama.Error
	if errors.As(err, &oerr) {
		return &struct {
			Code, Message string
			Details       any
		}{oerr.Code, oerr.Message, oerr.Details}
	}
	var merr *media.Error
	if errors.As(err, &merr) {
		return &struct {
			Code, Message string
			Details       any
		}{merr.Code, merr.Message, merr.Details}
	}
	var perr *prompt.Error
	if errors.As(err, &perr) {
		return &struct {
			Code, Message string
			Details       any
		}{perr.Code, perr.Message, perr.Details}
	}
	var eerr *engine.Error
	if errors.As(err, &eerr) {
		return &struct {
			Code, Message string
			Details       any
		}{eerr.Code, eerr.Message, eerr.Details}
	}
	return nil
}
