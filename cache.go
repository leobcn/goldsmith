package goldsmith

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
)

type fileCache struct {
	baseDir string
}

func (c *fileCache) retrieveFile(context *Context, outputPath string, inputFile *File) (*File, error) {
	cachePath := c.buildCachePath(context, outputPath)

	outputFile, err := NewFileFromAsset(outputPath, cachePath)
	if err != nil {
		return nil, err
	}

	if inputFile != nil && inputFile.ModTime().After(outputFile.ModTime()) {
		return nil, nil
	}

	return outputFile, nil
}

func (c *fileCache) storeFile(context *Context, outputFile *File) error {
	cachePath := c.buildCachePath(context, outputFile.Path())

	if err := os.MkdirAll(c.baseDir, 0755); err != nil {
		return err
	}

	fp, err := os.Create(cachePath)
	if err != nil {
		return err
	}
	defer fp.Close()

	if _, err := outputFile.Seek(0, os.SEEK_SET); err != nil {
		return err
	}

	if _, err := outputFile.WriteTo(fp); err != nil {
		return err
	}

	return nil
}

func (c *fileCache) buildCachePath(context *Context, path string) string {
	hashBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(hashBytes, context.hash)

	hash := crc32.NewIEEE()
	hash.Write(hashBytes)
	hash.Write([]byte(path))

	return filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x.%s", hash.Sum32(), filepath.Ext(path)))
}
