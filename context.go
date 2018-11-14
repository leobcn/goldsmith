package goldsmith

import (
	"os"
	"runtime"
	"sync"
)

type Context struct {
	gs *Goldsmith

	plugin Plugin
	hash   uint32

	fileFilters []Filter
	inputFiles  chan *File
	outputFiles chan *File
}

func (ctx *Context) DispatchFile(f *File) {
	ctx.outputFiles <- f
}

func (ctx *Context) CacheFile(inputFile, outputFile *File, depPaths ...string) {
	err := ctx.gs.cacheWriteFile(ctx, inputFile, outputFile, depPaths)
	if err != nil {
		ctx.gs.fault(ctx.plugin.Name(), outputFile, err)
	}
}

func (ctx *Context) step() {
	defer close(ctx.outputFiles)

	var err error
	var filters []Filter
	if initializer, ok := ctx.plugin.(Initializer); ok {
		filters, err = initializer.Initialize(ctx)
		if err != nil {
			ctx.gs.fault(ctx.plugin.Name(), nil, err)
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
				for file := range ctx.inputFiles {
					accept := processor != nil
					for _, filter := range append(ctx.fileFilters, filters...) {
						if accept, err = filter.Accept(ctx, file); err != nil {
							ctx.gs.fault(filter.Name(), file, err)
							return
						}

						if !accept {
							break
						}
					}

					if accept {
						if _, err := file.Seek(0, os.SEEK_SET); err != nil {
							ctx.gs.fault("core", file, err)
						}
						if err := processor.Process(ctx, file); err != nil {
							ctx.gs.fault(ctx.plugin.Name(), file, err)
						}
					} else {
						ctx.outputFiles <- file
					}
				}
			}()
		}
		wg.Wait()
	}

	if finalizer, ok := ctx.plugin.(Finalizer); ok {
		if err := finalizer.Finalize(ctx); err != nil {
			ctx.gs.fault(ctx.plugin.Name(), nil, err)
		}
	}
}
