package media

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AugustLigh/GoMino/internal/middleware"
	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/gofiber/fiber/v3"
)

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
	// Get configuration from context
	cfg := middleware.GetConfigFromContext(c)
	if cfg == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	var inputStream io.ReadSeeker
	var fileExt string

	contentType := c.Get("Content-Type")

	// 1. Try to get data from multipart or raw body
	if strings.HasPrefix(contentType, "multipart/form-data") {
		file, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
		}
		src, err := file.Open()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
		}
		defer src.Close()
		inputStream = src
		fileExt = filepath.Ext(file.Filename)
	} else {
		// Raw binary body (like in AminoLightPy)
		body := c.Body()
		if len(body) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
		}
		inputStream = bytes.NewReader(body)

		// Guess extension from Content-Type header
		if contentType != "" && !strings.Contains(contentType, "*") {
			parts := strings.Split(contentType, "/")
			if len(parts) > 1 {
				fileExt = "." + parts[1]
			}
		}
		// Fallback to byte detection if still empty
		if fileExt == "" {
			mimeType := http.DetectContentType(body)
			fileExt = ".bin"
			if parts := strings.Split(mimeType, "/"); len(parts) > 1 {
				fileExt = "." + parts[1]
			}
		}
	}

	// 2. MD5 hashing for deduplication
	if _, err := inputStream.Seek(0, 0); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}
	hash := md5.New()
	if _, err := io.Copy(hash, inputStream); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}
	md5String := hex.EncodeToString(hash.Sum(nil))

	// Detect MIME type properly
	if _, err := inputStream.Seek(0, 0); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}
	buffer := make([]byte, 512)
	n, err := inputStream.Read(buffer)
	if err != nil && err != io.EOF {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}
	mimeType := http.DetectContentType(buffer[:n])

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
	case strings.HasPrefix(mimeType, "audio/"):
		outputExt = ".mp3"
		ffmpegArgs = []string{"-i", "TEMP_FILE_PLACEHOLDER", "-vn", "-acodec", "libmp3lame", "-q:a", "2", "-y", "OUTPUT_FILE_PLACEHOLDER"}
	case strings.HasPrefix(mimeType, "video/"):
		outputExt = ".mp4"
		ffmpegArgs = []string{"-i", "TEMP_FILE_PLACEHOLDER", "-c:v", "libx264", "-c:a", "aac", "-y", "OUTPUT_FILE_PLACEHOLDER"}
	default:
		// Unsupported type
		return c.Status(fiber.StatusBadRequest).JSON(response.NewError(response.StatusInvalidRequest))
	}

	finalFileName := fmt.Sprintf("%s%s", md5String, outputExt)
	finalFilePath := filepath.Join(cfg.Media.StoragePath, finalFileName)

	// 3. Return existing if already converted
	if _, err := os.Stat(finalFilePath); err == nil {
		return c.Status(fiber.StatusOK).JSON(UploadResponse{
			MediaValue: fmt.Sprintf("%s%s", cfg.Media.ServerURL, finalFileName),
		})
	}

	// 4. Save to temp file for FFmpeg
	if _, err := inputStream.Seek(0, 0); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	// Use the correct extension for the temp file to help ffmpeg probe it
	tempExt := ".bin"
	exts, _ := checkMimeTypeExtension(mimeType)
	if len(exts) > 0 {
		tempExt = exts[0]
	}

	tempFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("temp_%s%s", md5String, tempExt))
	out, err := os.Create(tempFilePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}
	if _, err := io.Copy(out, inputStream); err != nil {
		out.Close()
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}
	out.Close()
	defer os.Remove(tempFilePath)

	// 5. Run FFmpeg
	// Replace placeholders
	for i, v := range ffmpegArgs {
		if v == "TEMP_FILE_PLACEHOLDER" {
			ffmpegArgs[i] = tempFilePath
		} else if v == "OUTPUT_FILE_PLACEHOLDER" {
			ffmpegArgs[i] = finalFilePath
		}
	}

	cmd := exec.Command("ffmpeg", ffmpegArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("FFmpeg Error: %v, Output: %s\n", err, string(out))
		// Check if it's a specific error we can report? For now generic 500.
		return c.Status(fiber.StatusInternalServerError).JSON(response.NewError(response.StatusServerError))
	}

	return c.Status(fiber.StatusOK).JSON(UploadResponse{
		MediaValue: fmt.Sprintf("%s%s", cfg.Media.ServerURL, finalFileName),
	})
}

// Helper to guess extension from mime (simple wrapper or just use a map)
func checkMimeTypeExtension(mime string) ([]string, error) {
	// Simple map for common types to avoid heavy dependencies if needed,
	// or use mime.ExtensionsByType if imported.
	// Since we didn't import "mime", let's do a basic switch or import it.
	// For this snippet, I'll rely on a basic switch or add the import.
	// Actually, let's just stick to a simple mapping here to avoid import issues in the snippet.
	switch {
	case strings.Contains(mime, "jpeg"):
		return []string{".jpg"}, nil
	case strings.Contains(mime, "png"):
		return []string{".png"}, nil
	case strings.Contains(mime, "gif"):
		return []string{".gif"}, nil
	case strings.Contains(mime, "webp"):
		return []string{".webp"}, nil
	case strings.Contains(mime, "mp3"):
		return []string{".mp3"}, nil
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
