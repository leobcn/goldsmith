package goldsmith

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"os"
	"path/filepath"
)

type cache struct {
	baseDir string
}

type cacheEntry struct {
	Meta     map[string]interface{}
	RelPath  string
	DepPaths []string
}

func (c *cache) buildCachePaths(context *Context, file *File) (string, string) {
	hashBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(hashBytes, context.hash)

	hash := crc32.NewIEEE()
	hash.Write(hashBytes)
	hash.Write([]byte(context.plugin.Name()))
	hash.Write([]byte(file.Path()))

	stateHash := hash.Sum32()
	dataPath := filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_data", stateHash))
	entryPath := filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_entry.json", stateHash))

	return dataPath, entryPath
}

func (c *cache) readFile(context *Context, inputFile *File) (*File, error) {
	if len(c.baseDir) == 0 {
		return nil, nil
	}

	dataPath, entryPath := c.buildCachePaths(context, inputFile)

	entry, err := c.readFileEntry(entryPath)
	if err != nil {
		return nil, err
	}

	outputFile, err := NewFileFromAsset(entry.RelPath, dataPath)
	if err != nil {
		return nil, err
	}
	if inputFile.ModTime().After(inputFile.ModTime()) {
		return nil, nil
	}

	outputFile.Meta = entry.Meta
	return outputFile, nil
}

func (c *cache) readFileEntry(path string) (*cacheEntry, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

func (c *cache) writeFile(context *Context, inputFile, outputFile *File, depPaths []string) error {
	if len(c.baseDir) == 0 {
		return nil
	}

	dataPath, entryPath := c.buildCachePaths(context, inputFile)

	if err := c.writeFileData(dataPath, outputFile); err != nil {
		return err
	}

	if err := c.writeFileEntry(entryPath, outputFile, depPaths); err != nil {
		return err
	}

	return nil
}

func (c *cache) writeFileData(path string, file *File) error {
	fp, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fp.Close()

	if _, err := file.WriteTo(fp); err != nil {
		return err
	}

	return nil
}

func (c *cache) writeFileEntry(path string, file *File, depPaths []string) error {
	entry := cacheEntry{
		Meta:     file.Meta,
		RelPath:  file.Path(),
		DepPaths: depPaths,
	}

	json, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, json, 0666)
}
