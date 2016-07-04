/*
 * Copyright (c) 2015 Alex Yatskov <alex@foosoft.net>
 * Author: Alex Yatskov <alex@foosoft.net>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package goldsmith

import (
	"os"
	"path/filepath"
	"sync"
)

type goldsmith struct {
	srcDir, dstDir string
	contexts       []*context
	refs           map[string]bool

	errors   []error
	errorMtx sync.Mutex
}

func (gs *goldsmith) pushContext(plug Plugin, filters []string) *context {
	ctx := &context{
		gs:      gs,
		plug:    plug,
		filters: filters,
		output:  make(chan *file),
	}

	if len(gs.contexts) > 0 {
		ctx.input = gs.contexts[len(gs.contexts)-1].output
	}

	gs.contexts = append(gs.contexts, ctx)
	return ctx
}

func (gs *goldsmith) cleanupFiles() {
	infos := make(chan fileInfo)
	go scanDir(gs.dstDir, infos)

	for info := range infos {
		relPath, _ := filepath.Rel(gs.dstDir, info.path)
		if contained, _ := gs.refs[relPath]; contained {
			continue
		}

		os.RemoveAll(info.path)
	}
}

func (gs *goldsmith) exportFile(f *file) error {
	if err := f.export(gs.dstDir); err != nil {
		return err
	}

	pathSeg := cleanPath(f.path)
	for {
		gs.refs[pathSeg] = true
		if pathSeg == "." {
			break
		}

		pathSeg = filepath.Dir(pathSeg)
	}

	return nil
}

func (gs *goldsmith) fault(f *file, err error) {
	gs.errorMtx.Lock()
	defer gs.errorMtx.Unlock()

	ferr := &Error{Err: err}
	if f != nil {
		ferr.Path = f.path
	}

	gs.errors = append(gs.errors, ferr)
}

//
//	Goldsmith Implementation
//

func (gs *goldsmith) Chain(p Plugin, filters ...string) Goldsmith {
	gs.pushContext(p, filters)
	return gs
}

func (gs *goldsmith) End(dstDir string) []error {
	gs.dstDir = dstDir

	for _, ctx := range gs.contexts {
		go ctx.chain()
	}

	ctx := gs.contexts[len(gs.contexts)-1]
	for f := range ctx.output {
		gs.exportFile(f)
	}

	gs.cleanupFiles()
	return gs.errors
}
