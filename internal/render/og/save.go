package og

import (
	"fmt"
	"image"
	"image/jpeg"
	"os"
)

func saveJPEG(img image.Image, path string, quality int) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: quality}); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("encode jpeg: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
