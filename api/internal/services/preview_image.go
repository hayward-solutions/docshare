package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/disintegration/imaging"
	"github.com/docshare/api/internal/models"
	"github.com/google/uuid"

	// Register WebP decoder so imaging.Decode accepts .webp source files.
	// imaging itself only pulls in JPEG/PNG/GIF/BMP/TIFF; WebP encode is
	// not available in pure Go, but decode is — we re-emit as JPEG.
	_ "golang.org/x/image/webp"
)

const (
	imageThumbnailMaxDim       = 400
	imageThumbnailJPEGQuality  = 80
	imageThumbnailContentType  = "image/jpeg"
)

// IsThumbnailableImage reports whether a raster image with the given mime can
// be decoded by the pure-Go pipeline. SVG is intentionally excluded — it's
// already tiny and the frontend renders the original directly. HEIC/HEIF
// have no pure-Go decoder so we skip them too; the FileThumbnail UI will
// fall back to the file-type icon.
func IsThumbnailableImage(mime string) bool {
	switch mime {
	case "image/jpeg", "image/png", "image/gif",
		"image/webp", "image/bmp", "image/tiff":
		return true
	default:
		return false
	}
}

// renderImageThumbnail downloads the original image, resizes it to fit
// within imageThumbnailMaxDim×imageThumbnailMaxDim (preserving aspect),
// re-encodes as JPEG, uploads to S3, and publishes the path on the File
// row through the same race-guarded UPDATE used by the Office-doc path.
//
// On EXIF-tagged orientation, the decoded image is auto-rotated so phone
// photos don't come out sideways.
func (p *PreviewService) renderImageThumbnail(ctx context.Context, file *models.File, notAfter time.Time) (string, error) {
	sourceObject, err := p.Storage.Download(ctx, file.StoragePath)
	if err != nil {
		return "", err
	}
	defer sourceObject.Close()

	jpegBytes, err := resizeImageToJPEG(sourceObject, imageThumbnailMaxDim, imageThumbnailJPEGQuality)
	if err != nil {
		return "", err
	}

	previewPath := fmt.Sprintf("%s/previews/%s.jpg", file.OwnerID.String(), uuid.New().String())
	if err := p.Storage.Upload(ctx, previewPath, bytes.NewReader(jpegBytes), int64(len(jpegBytes)), imageThumbnailContentType); err != nil {
		return "", err
	}

	return p.publishThumbnail(ctx, file, previewPath, notAfter, imageThumbnailContentType)
}

// resizeImageToJPEG decodes an image from r, fits it into a maxDim×maxDim
// box (preserving aspect ratio), and re-encodes it as JPEG at the given
// quality (1-100). EXIF orientation is applied on decode so phone photos
// come out upright. The returned byte slice is the encoded JPEG.
func resizeImageToJPEG(r io.Reader, maxDim, quality int) ([]byte, error) {
	img, err := imaging.Decode(r, imaging.AutoOrientation(true))
	if err != nil {
		return nil, fmt.Errorf("image decode failed: %w", err)
	}

	resized := imaging.Fit(img, maxDim, maxDim, imaging.Lanczos)

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, resized, imaging.JPEG, imaging.JPEGQuality(quality)); err != nil {
		return nil, fmt.Errorf("image encode failed: %w", err)
	}
	return buf.Bytes(), nil
}
