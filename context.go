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

func (ctx *Context) DispatchFile(file *File) {
	ctx.outputFiles <- file
}

func (ctx *Context) DispatchFileAndCache(outputFile, inputFile *File, depPaths ...string) {
	ctx.gs.storeFile(ctx, outputFile, inputFile, depPaths)
	ctx.DispatchFile(outputFile)
}

func (ctx *Context) RetrieveCachedFile(inputFile *File) *File {
	return ctx.gs.retrieveFile(ctx, inputFile)
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
				for inputFile := range ctx.inputFiles {
					accept := processor != nil
					for _, filter := range append(ctx.fileFilters, filters...) {
						if accept, err = filter.Accept(ctx, inputFile); err != nil {
							ctx.gs.fault(filter.Name(), inputFile, err)
							return
						}

						if !accept {
							break
						}
					}

					if accept {
						if _, err := inputFile.Seek(0, os.SEEK_SET); err != nil {
							ctx.gs.fault("core", inputFile, err)
						}
						if err := processor.Process(ctx, inputFile); err != nil {
							ctx.gs.fault(ctx.plugin.Name(), inputFile, err)
						}
					} else {
						ctx.outputFiles <- inputFile
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
