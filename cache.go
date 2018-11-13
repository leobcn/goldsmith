package goldsmith

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"os"
	"path/filepath"
)

type cache struct {
	baseDir string
}

type cacheEntry struct {
	Meta map[string]interface{}

	Size     int64
	DataHash int32
	MetaHash int32
	ModTime  int64

	RelPath  string
	DepPaths []string
}

func (c *cache) buildCachePaths(name, path string) (string, string) {
	hash := fnv.New32a()
	hash.Write([]byte(name))
	hash.Write([]byte(path))
	sum := hash.Sum32()

	ext := filepath.Ext(path)

	dataPath := filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_data%s", sum, ext))
	entryPath := filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_entry.json", sum))

	return dataPath, entryPath
}

func (c *cache) readFile(pluginName string, inputFile *File) (*File, error) {
	if len(c.baseDir) == 0 {
		return nil, nil
	}

	dataPath, entryPath := c.buildCachePaths(pluginName, inputFile.Path())

	entry, err := c.readFileEntry(entryPath)
	if err != nil {
		return nil, err
	}

	if entry.Size != inputFile.ModTime().Unix() {
		return nil, nil
	}

	if entry.MetaHash != inputFile.MetaHash() {
		return nil, nil
	}

	if entry.DataHash != inputFile.DataHash() {
		return nil, nil
	}

	return NewFileFromAsset(entry.RelPath, dataPath)
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

func (c *cache) writeFile(pluginName string, inputFile, outputFile *File, depPaths []string) error {
	if len(c.baseDir) == 0 {
		return nil
	}

	dataPath, entryPath := c.buildCachePaths(pluginName, inputFile.Path())

	if err := c.writeFileData(dataPath, outputFile); err != nil {
		return err
	}

	if err := c.writeFileEntry(entryPath, inputFile, depPaths); err != nil {
		return err
	}

	return nil
}

func (c *cache) writeFileData(path string, f *File) error {
	fp, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fp.Close()

	if _, err := f.WriteTo(fp); err != nil {
		return err
	}

	return nil
}

func (c *cache) writeFileEntry(path string, file *File, depPaths []string) error {
	entry := cacheEntry{
		Meta:     file.Meta,
		Size:     file.Size(),
		DataHash: file.DataHash(),
		MetaHash: file.MetaHash(),
		ModTime:  file.ModTime().Unix(),
		RelPath:  file.Path(),
		DepPaths: depPaths,
	}

	json, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, json, 0666)
}
