package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/AugustLigh/GoMino/internal/middleware"
	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/gofiber/fiber/v3"
)

const (
	// MaxFileSize is the maximum allowed file size (50 MB)
	MaxFileSize = 50 * 1024 * 1024
	// FFmpegTimeout is the maximum time FFmpeg can run
	FFmpegTimeout = 2 * time.Minute
)

// allowedMimeTypes is the whitelist of accepted MIME types
var allowedMimeTypes = map[string]bool{
	"image/jpeg":    true,
	"image/png":     true,
	"image/gif":     true,
	"image/webp":    true,
	"image/bmp":     true,
	"audio/mpeg":    true,
	"audio/mp3":     true,
	"audio/aac":     true,
	"audio/mp4":     true,
	"audio/x-m4a":   true,
	"audio/ogg":     true,
	"audio/wav":     true,
	"audio/x-wav":   true,
	"video/mp4":     true,
	"video/webm":    true,
	"video/x-msvideo": true,
	"video/quicktime": true,
}

// isAllowedMimePrefix checks if the MIME type prefix is allowed
func isAllowedMimePrefix(mimeType string) bool {
	if allowedMimeTypes[mimeType] {
		return true
	}
	// Check prefixes for common types
	return strings.HasPrefix(mimeType, "image/") ||
		strings.HasPrefix(mimeType, "audio/") ||
		strings.HasPrefix(mimeType, "video/")
}

// UploadMedia godoc
// @Summary Upload media file
// @Description Upload media (image/video) and convert to WebP
// @Tags media
// @Accept  multipart/form-data
// @Produce  json
// @Param   file formData file true "Media file"
// @Success 200 {object} UploadResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /g/s/media/upload [post]
func UploadMedia(c fiber.Ctx) error {
	// Rate limit media uploads: reuse general limiter per user
	auid := middleware.GetAUIDFromContext(c)
	if auid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(response.NewError(response.StatusNoPermission))
	}

	// Get configuration from context
	cfg := middleware.GetConfigFromContext(c)
	if cfg == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Validate storage path exists
	if _, err := os.Stat(cfg.Media.StoragePath); os.IsNotExist(err) {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	var inputStream io.ReadSeeker
	var dataSize int64

	contentType := c.Get("Content-Type")

	// 1. Try to get data from multipart or raw body
	if strings.HasPrefix(contentType, "multipart/form-data") {
		file, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
		}
		// Check file size
		if file.Size > MaxFileSize {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
				response.StatusInvalidRequest, "File too large. Maximum size is 50 MB."))
		}
		dataSize = file.Size
		src, err := file.Open()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
		}
		defer src.Close()
		inputStream = src
	} else {
		// Raw binary body (like in AminoLightPy)
		body := c.Body()
		if len(body) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
		}
		if int64(len(body)) > MaxFileSize {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
				response.StatusInvalidRequest, "File too large. Maximum size is 50 MB."))
		}
		dataSize = int64(len(body))
		inputStream = bytes.NewReader(body)
	}

	// Reject empty files
	if dataSize == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	// 2. SHA-256 hashing for deduplication (MD5 is cryptographically broken)
	if _, err := inputStream.Seek(0, 0); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, inputStream); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}
	hashString := hex.EncodeToString(hash.Sum(nil))

	// Detect MIME type from file content (magic bytes), NOT from headers
	if _, err := inputStream.Seek(0, 0); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}
	buffer := make([]byte, 512)
	n, err := inputStream.Read(buffer)
	if err != nil && err != io.EOF {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}
	mimeType := http.DetectContentType(buffer[:n])

	// For audio formats that http.DetectContentType can't detect (returns octet-stream),
	// only accept if magic bytes show valid audio signatures
	if mimeType == "application/octet-stream" {
		// Check for known audio magic bytes
		detectedAudio := detectAudioFormat(buffer[:n])
		if detectedAudio != "" {
			mimeType = detectedAudio
		} else {
			// Reject unknown binary files
			return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
				response.StatusInvalidRequest, "Unsupported file format."))
		}
	}

	// Validate the detected MIME type is allowed
	if !isAllowedMimePrefix(mimeType) {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewErrorWithMessage(
			response.StatusInvalidRequest, "Unsupported file format."))
	}

	// Determine output extension and ffmpeg args
	var outputExt string
	var ffmpegArgs []string

	switch {
	case strings.HasPrefix(mimeType, "image/gif"):
		outputExt = ".webp"
		ffmpegArgs = []string{"-i", "TEMP_FILE_PLACEHOLDER", "-c:v", "libwebp_anim", "-lossless", "0", "-quality", "80", "-loop", "0", "-y", "OUTPUT_FILE_PLACEHOLDER"}
	case strings.HasPrefix(mimeType, "image/"):
		outputExt = ".webp"
		ffmpegArgs = []string{"-i", "TEMP_FILE_PLACEHOLDER", "-c:v", "libwebp", "-quality", "80", "-y", "OUTPUT_FILE_PLACEHOLDER"}
	case strings.HasPrefix(mimeType, "audio/") && (strings.Contains(mimeType, "mp4") || strings.Contains(mimeType, "aac") || strings.Contains(mimeType, "m4a")):
		outputExt = ".m4a"
		ffmpegArgs = []string{"-i", "TEMP_FILE_PLACEHOLDER", "-c", "copy", "-y", "OUTPUT_FILE_PLACEHOLDER"}
	case strings.HasPrefix(mimeType, "audio/"):
		outputExt = ".mp3"
		ffmpegArgs = []string{"-i", "TEMP_FILE_PLACEHOLDER", "-vn", "-acodec", "libmp3lame", "-q:a", "2", "-y", "OUTPUT_FILE_PLACEHOLDER"}
	case strings.HasPrefix(mimeType, "video/"):
		outputExt = ".mp4"
		ffmpegArgs = []string{
			"-i", "TEMP_FILE_PLACEHOLDER",
			"-c:v", "libx264",
			"-profile:v", "baseline",
			"-level", "3.1",
			"-pix_fmt", "yuv420p",
			"-preset", "fast",
			"-crf", "28",
			"-vf", "scale='min(1280,iw)':'min(720,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2,setsar=1",
			"-c:a", "aac",
			"-b:a", "128k",
			"-ac", "2",
			"-movflags", "+faststart",
			"-y", "OUTPUT_FILE_PLACEHOLDER",
		}
	default:
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	finalFileName := fmt.Sprintf("%s%s", hashString, outputExt)
	finalFilePath := filepath.Join(cfg.Media.StoragePath, finalFileName)

	// Validate the final path is still within storage directory (prevent path traversal)
	absStorage, err := filepath.Abs(cfg.Media.StoragePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}
	absFinal, err := filepath.Abs(finalFilePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}
	if !strings.HasPrefix(absFinal, absStorage+string(os.PathSeparator)) && absFinal != absStorage {
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	// 3. Return existing if already converted (deduplication)
	if _, err := os.Stat(finalFilePath); err == nil {
		return c.Status(fiber.StatusOK).JSON(UploadResponse{
			MediaValue: fmt.Sprintf("%s%s", cfg.Media.ServerURL, finalFileName),
		})
	}

	// 4. Save to temp file for FFmpeg — use private temp directory
	if _, err := inputStream.Seek(0, 0); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Use the correct extension for the temp file to help ffmpeg probe it
	tempExt := ".bin"
	exts, _ := checkMimeTypeExtension(mimeType)
	if len(exts) > 0 {
		tempExt = exts[0]
	}

	// Create temp file with restrictive permissions
	tempFile, err := os.CreateTemp("", fmt.Sprintf("gomino_upload_*%s", tempExt))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}
	tempFilePath := tempFile.Name()
	// Ensure cleanup happens regardless of how we exit
	defer os.Remove(tempFilePath)

	// Set restrictive permissions
	if err := tempFile.Chmod(0600); err != nil {
		tempFile.Close()
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	if _, err := io.Copy(tempFile, inputStream); err != nil {
		tempFile.Close()
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}
	tempFile.Close()

	// 5. Run FFmpeg with timeout
	for i, v := range ffmpegArgs {
		if v == "TEMP_FILE_PLACEHOLDER" {
			ffmpegArgs[i] = tempFilePath
		} else if v == "OUTPUT_FILE_PLACEHOLDER" {
			ffmpegArgs[i] = finalFilePath
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), FFmpegTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg", ffmpegArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Clean up partial output file on error
		os.Remove(finalFilePath)
		fmt.Printf("FFmpeg Error: %v, Output: %s\n", err, string(output))
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Verify output file was created
	if _, err := os.Stat(finalFilePath); os.IsNotExist(err) {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.Status(fiber.StatusOK).JSON(UploadResponse{
		MediaValue: fmt.Sprintf("%s%s", cfg.Media.ServerURL, finalFileName),
	})
}

// detectAudioFormat checks magic bytes for audio formats not detected by http.DetectContentType
func detectAudioFormat(data []byte) string {
	if len(data) < 4 {
		return ""
	}
	// AAC ADTS header: starts with 0xFF 0xF1 or 0xFF 0xF9
	if data[0] == 0xFF && (data[1]&0xF0) == 0xF0 {
		return "audio/aac"
	}
	// M4A/MP4 container: "ftyp" at offset 4
	if len(data) >= 8 && string(data[4:8]) == "ftyp" {
		return "audio/mp4"
	}
	// OGG: "OggS"
	if string(data[:4]) == "OggS" {
		return "audio/ogg"
	}
	// FLAC: "fLaC"
	if string(data[:4]) == "fLaC" {
		return "audio/flac"
	}
	// WAV: "RIFF"
	if string(data[:4]) == "RIFF" && len(data) >= 12 && string(data[8:12]) == "WAVE" {
		return "audio/wav"
	}
	return ""
}

// checkMimeTypeExtension maps MIME types to file extensions
func checkMimeTypeExtension(mime string) ([]string, error) {
	switch {
	case strings.Contains(mime, "jpeg"):
		return []string{".jpg"}, nil
	case strings.Contains(mime, "png"):
		return []string{".png"}, nil
	case strings.Contains(mime, "gif"):
		return []string{".gif"}, nil
	case strings.Contains(mime, "webp"):
		return []string{".webp"}, nil
	case strings.Contains(mime, "mp3") || mime == "audio/mpeg":
		return []string{".mp3"}, nil
	case strings.Contains(mime, "aac"):
		return []string{".aac"}, nil
	case strings.HasPrefix(mime, "audio/") && strings.Contains(mime, "mp4"):
		return []string{".m4a"}, nil
	case strings.Contains(mime, "m4a"):
		return []string{".m4a"}, nil
	case strings.Contains(mime, "ogg"):
		return []string{".ogg"}, nil
	case strings.Contains(mime, "wav"):
		return []string{".wav"}, nil
	case strings.Contains(mime, "mp4"):
		return []string{".mp4"}, nil
	default:
		return []string{".bin"}, nil
	}
}
