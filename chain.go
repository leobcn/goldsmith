package goldsmith

import (
	"os"
	"path/filepath"
	"sync"
)

type chain struct {
	srcDir string
	dstDir string

	links    []*link
	refs     map[string]bool
	filters  []Filter
	complete bool

	errors   []error
	errorMtx sync.Mutex
}

func (c *chain) linkPlugin(plug Plugin) *link {
	ctx := &link{chain: c, plugin: plug, output: make(chan *file)}
	ctx.filters = append(ctx.filters, c.filters...)

	if len(c.links) > 0 {
		ctx.input = c.links[len(c.links)-1].output
	}

	c.links = append(c.links, ctx)
	return ctx
}

func (c *chain) cleanupFiles() {
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

func (c *chain) exportFile(f *file) error {
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

func (c *chain) fault(name string, f *file, err error) {
	c.errorMtx.Lock()
	defer c.errorMtx.Unlock()

	ferr := &Error{Name: name, Err: err}
	if f != nil {
		ferr.Path = f.path
	}

	c.errors = append(c.errors, ferr)
}

//
//	Goldsmith Implementation
//

func (c *chain) Chain(p Plugin) Goldsmith {
	if c.complete {
		panic("attempted reuse of goldsmith instance")
	}

	c.linkPlugin(p)
	return c
}

func (c *chain) FilterPush(f Filter) Goldsmith {
	if c.complete {
		panic("attempted reuse of goldsmith instance")
	}

	c.filters = append(c.filters, f)
	return c
}

func (c *chain) FilterPop() Goldsmith {
	if c.complete {
		panic("attempted reuse of goldsmith instance")
	}

	count := len(c.filters)
	if count == 0 {
		panic("attempted to pop empty filter stack")
	}

	c.filters = c.filters[:count-1]
	return c
}

func (c *chain) End(dstDir string) []error {
	if c.complete {
		panic("attempted reuse of goldsmith instance")
	}

	c.dstDir = dstDir

	for _, ctx := range c.links {
		go ctx.step()
	}

	ctx := c.links[len(c.links)-1]
	for f := range ctx.output {
		c.exportFile(f)
	}

	c.cleanupFiles()
	c.complete = true

	return c.errors
}
