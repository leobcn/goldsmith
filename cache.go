package goldsmith

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
)

type cache struct {
	baseDir string
}

func (c *cache) buildCachePaths(name, path string) (dataPath, metaPath, depsPath string) {
	h := fnv.New32a()
	h.Write([]byte(name))
	h.Write([]byte(path))
	sum := h.Sum32()

	ext := filepath.Ext(path)

	dataPath = filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_data%s", sum, ext))
	metaPath = filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_meta.json", sum))
	depsPath = filepath.Join(c.baseDir, fmt.Sprintf("gs_%.8x_deps.txt", sum))
	return
}

func (c *cache) writeFile(name string, f *file, deps []string) error {
	if len(deps) == 0 {
		panic("cached files must have one or more dependencies")
	}

	if len(c.baseDir) > 0 {
		dataPath, metaPath, depsPath := c.buildCachePaths(name, f.Path())
		if err := c.writeFileData(dataPath, f); err != nil {
			return err
		}
		if err := c.writeFileMeta(metaPath, f); err != nil {
			return err
		}
		if err := c.writeFileDeps(depsPath, deps); err != nil {
			return err
		}
	}

	return nil
}

func (c *cache) writeFileData(path string, f *file) error {
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

func (c *cache) writeFileMeta(path string, f *file) error {
	metaJson, err := json.Marshal(f.Meta)
	if err != nil {
		return err
	}

	fp, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fp.Close()

	if _, err := fp.Write(metaJson); err != nil {
		return err
	}

	return nil
}

func (c *cache) writeFileDeps(path string, deps []string) error {
	fp, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fp.Close()

	for _, dep := range deps {
		if _, err := fp.WriteString(fmt.Sprintln(dep)); err != nil {
			return err
		}
	}

	return nil
}
