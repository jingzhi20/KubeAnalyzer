package response

import (
	"time"

	"github.com/gin-gonic/gin"
)

// ErrorDetail represents the structured error information.
type ErrorDetail struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
	Timestamp  string `json:"timestamp"`
}

// ErrorResponse wraps ErrorDetail under the "error" key.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// NewError sends a JSON error response with the given HTTP status code.
func NewError(c *gin.Context, httpStatus int, code, message, suggestion string) {
	c.JSON(httpStatus, ErrorResponse{
		Error: ErrorDetail{
			Code:       code,
			Message:    message,
			Suggestion: suggestion,
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// BadRequest sends a 400 error response.
func BadRequest(c *gin.Context, code, message, suggestion string) {
	NewError(c, 400, code, message, suggestion)
}

// Unauthorized sends a 401 error response.
func Unauthorized(c *gin.Context, code, message, suggestion string) {
	NewError(c, 401, code, message, suggestion)
}

// Forbidden sends a 403 error response.
func Forbidden(c *gin.Context, code, message, suggestion string) {
	NewError(c, 403, code, message, suggestion)
}

// NotFound sends a 404 error response.
func NotFound(c *gin.Context, code, message, suggestion string) {
	NewError(c, 404, code, message, suggestion)
}

// Conflict sends a 409 error response.
func Conflict(c *gin.Context, code, message, suggestion string) {
	NewError(c, 409, code, message, suggestion)
}

// UnprocessableEntity sends a 422 error response.
func UnprocessableEntity(c *gin.Context, code, message, suggestion string) {
	NewError(c, 422, code, message, suggestion)
}

// InternalError sends a 500 error response.
func InternalError(c *gin.Context, code, message, suggestion string) {
	NewError(c, 500, code, message, suggestion)
}

// ServiceUnavailable sends a 503 error response.
func ServiceUnavailable(c *gin.Context, code, message, suggestion string) {
	NewError(c, 503, code, message, suggestion)
}

// BadGateway sends a 502 error response.
func BadGateway(c *gin.Context, code, message, suggestion string) {
	NewError(c, 502, code, message, suggestion)
}
