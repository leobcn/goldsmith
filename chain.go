package goldsmith

import (
	"os"
	"path/filepath"
)

func (c *Goldsmith) linkPlugin(plug Plugin) *link {
	ctx := &link{chain: c, plugin: plug, output: make(chan *File)}
	ctx.filters = append(ctx.filters, c.filters...)

	if len(c.links) > 0 {
		ctx.input = c.links[len(c.links)-1].output
	}

	c.links = append(c.links, ctx)
	return ctx
}

func (c *Goldsmith) cleanupFiles() {
	infos := make(chan fileInfo)
	go scanDir(c.dstDir, infos)

	for info := range infos {
		relPath, _ := filepath.Rel(c.dstDir, info.path)
		if contained, _ := c.refs[relPath]; contained {
			continue
		}

		os.RemoveAll(info.path)
	}
}

func (c *Goldsmith) exportFile(f *File) error {
	if err := f.export(c.dstDir); err != nil {
		return err
	}

	pathSeg := cleanPath(f.path)
	for {
		c.refs[pathSeg] = true
		if pathSeg == "." {
			break
		}

		pathSeg = filepath.Dir(pathSeg)
	}

	return nil
}

func (c *Goldsmith) fault(name string, f *File, err error) {
	c.errorMtx.Lock()
	defer c.errorMtx.Unlock()

	ferr := &Error{Name: name, Err: err}
	if f != nil {
		ferr.Path = f.path
	}

	c.errors = append(c.errors, ferr)
}

func (c *Goldsmith) cacheWriteFile(pluginName string, inputFile, outputFile *File, depPaths []string) error {
	return c.cache.writeFile(pluginName, inputFile, outputFile, depPaths)
}

func (c *Goldsmith) cacheReadFile(pluginName string, inputFile *File) (*File, error) {
	outputFile, err := c.cache.readFile(pluginName, inputFile)
	if err != nil {
		return nil, err
	}

	return outputFile, nil
}
