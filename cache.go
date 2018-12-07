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

func (c *fileCache) retrieveFile(context *Context, outputPath string, inputFiles []*File) (*File, error) {
	cachePath, err := c.buildCachePath(context, outputPath, inputFiles)
	if err != nil {
		return nil, err
	}

	outputFile, err := NewFileFromAsset(outputPath, cachePath)
	if err != nil {
		return nil, err
	}

	return outputFile, nil
}

func (c *fileCache) storeFile(context *Context, outputFile *File, inputFiles []*File) error {
	cachePath, err := c.buildCachePath(context, outputFile.Path(), inputFiles)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(c.baseDir, 0755); err != nil {
		return err
	}

	fp, err := os.Create(cachePath)
	if err != nil {
		return err
	}
	defer fp.Close()

	offset, err := outputFile.Seek(0, os.SEEK_CUR)
	if err != nil {
		return err
	}

	if _, err := outputFile.Seek(0, os.SEEK_SET); err != nil {
		return err
	}

	if _, err := outputFile.WriteTo(fp); err != nil {
		return err
	}

	if _, err := outputFile.Seek(offset, os.SEEK_SET); err != nil {
		return err
	}

	return nil
}

func (c *fileCache) buildCachePath(context *Context, outputPath string, inputFiles []*File) (string, error) {
	uintBuff := make([]byte, 4)

	hash := crc32.NewIEEE()
	binary.LittleEndian.PutUint32(uintBuff, context.hash)
	hash.Write(uintBuff)
	hash.Write([]byte(outputPath))

	for _, inputFile := range inputFiles {
		fileHash, err := inputFile.hash()
		if err == nil {
			binary.LittleEndian.PutUint32(uintBuff, fileHash)
			hash.Write(uintBuff)
			hash.Write([]byte(inputFile.Path()))
		} else {
			return "", err
		}
	}

	cachePath := filepath.Join(c.baseDir, fmt.Sprintf(
		"gs_%.8x%s",
		hash.Sum32(),
		filepath.Ext(outputPath),
	))

	return cachePath, nil
}
