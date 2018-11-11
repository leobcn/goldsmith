package goldsmith

import (
	"os"
	"runtime"
	"sync"
)

type Context struct {
	chain  *Goldsmith
	plugin Plugin

	fileFilters []Filter
	inputFiles  chan *File
	outputFiles chan *File
}

func (ctx *Context) DispatchFile(f *File) {
	ctx.outputFiles <- f
}

func (ctx *Context) CacheFile(inputFile, outputFile *File, depPaths ...string) {
	err := ctx.chain.cacheWriteFile(ctx.plugin.Name(), inputFile, outputFile, depPaths)
	if err != nil {
		ctx.chain.fault(ctx.plugin.Name(), outputFile, err)
	}
}

func (ctx *Context) SrcDir() string {
	return ctx.chain.sourceDir
}

func (ctx *Context) DstDir() string {
	return ctx.chain.targetDir
}

func (ctx *Context) step() {
	defer close(ctx.outputFiles)

	var err error
	var filters []Filter
	if initializer, ok := ctx.plugin.(Initializer); ok {
		filters, err = initializer.Initialize(ctx)
		if err != nil {
			ctx.chain.fault(ctx.plugin.Name(), nil, err)
			return
		}
	}

	if ctx.inputFiles != nil {
		processor, _ := ctx.plugin.(Processor)

		var wg sync.WaitGroup
		for i := 0; i < runtime.NumCPU(); i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for f := range ctx.inputFiles {
					accept := processor != nil
					for _, filter := range append(ctx.fileFilters, filters...) {
						if accept, err = filter.Accept(ctx, f); err != nil {
							ctx.chain.fault(filter.Name(), f, err)
							return
						}

						if !accept {
							break
						}
					}

					if accept {
						if _, err := f.Seek(0, os.SEEK_SET); err != nil {
							ctx.chain.fault("core", f, err)
						}
						if err := processor.Process(ctx, f); err != nil {
							ctx.chain.fault(ctx.plugin.Name(), f, err)
						}
					} else {
						ctx.outputFiles <- f
					}
				}
			}()
		}
		wg.Wait()
	}

	if finalizer, ok := ctx.plugin.(Finalizer); ok {
		if err := finalizer.Finalize(ctx); err != nil {
			ctx.chain.fault(ctx.plugin.Name(), nil, err)
		}
	}
}
