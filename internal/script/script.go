// Package script runs FlClash's pinned QuickJS engine as an isolated WASI
// module. It grants no filesystem, process, network, or timer bindings to JS.
package script

import (
	"context"
	"crypto/rand"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed quickjs.wasm
var engine []byte

var shared struct {
	sync.Once
	runtime wazero.Runtime
	module  wazero.CompiledModule
	err     error
}

func prepare() (wazero.Runtime, wazero.CompiledModule, error) {
	shared.Do(func() {
		ctx := context.Background()
		shared.runtime = wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCloseOnContextDone(true).WithMemoryLimitPages(4352))
		if _, shared.err = wasi_snapshot_preview1.Instantiate(ctx, shared.runtime); shared.err != nil {
			return
		}
		shared.module, shared.err = shared.runtime.CompileModule(ctx, engine)
	})
	return shared.runtime, shared.module, shared.err
}

func Evaluate(source string, config []byte) ([]byte, error) {
	return evaluateWithin(source, config, 10*time.Second)
}
func evaluateWithin(source string, config []byte, timeout time.Duration) ([]byte, error) {
	if strings.ContainsRune(source, 0) || strings.ContainsRune(string(config), 0) {
		return nil, errors.New("script and profile cannot contain NUL bytes")
	}
	runtime, compiled, err := prepare()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var logs limitedLog
	defer func() {
		if logs.text.Len() > 0 {
			log.Print(strings.TrimSpace(logs.text.String()))
		}
	}()
	module, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName("").WithStartFunctions("_initialize").WithStderr(&logs).WithSysWalltime().WithSysNanotime().WithRandSource(rand.Reader))
	if err != nil {
		return nil, err
	}
	defer module.Close(context.Background())
	allocate := func(data []byte) (uint64, error) {
		ptr, e := module.ExportedFunction("malloc").Call(ctx, uint64(len(data)+1))
		if e != nil {
			return 0, e
		}
		if len(ptr) != 1 || ptr[0] == 0 || !module.Memory().Write(uint32(ptr[0]), append(append([]byte(nil), data...), 0)) {
			return 0, errors.New("script input exceeds memory limit")
		}
		return ptr[0], nil
	}
	scriptPtr, err := allocate([]byte(source))
	if err != nil {
		return nil, err
	}
	configPtr, err := allocate(config)
	if err != nil {
		return nil, err
	}
	result, err := module.ExportedFunction("flclash_evaluate").Call(ctx, scriptPtr, configPtr)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("override script exceeded %s", timeout)
		}
		return nil, fmt.Errorf("QuickJS: %w", err)
	}
	size, err := module.ExportedFunction("flclash_result_size").Call(ctx)
	if err != nil {
		return nil, err
	}
	if len(result) != 1 || result[0] == 0 || len(size) != 1 || size[0] < 1 {
		return nil, errors.New("script exceeded memory limit")
	}
	bytes, ok := module.Memory().Read(uint32(result[0]), uint32(size[0]))
	if !ok {
		return nil, errors.New("invalid QuickJS response")
	}
	if bytes[0] != '0' {
		return nil, errors.New(string(bytes[1:]))
	}
	return append([]byte(nil), bytes[1:]...), nil
}

// Console output is bounded independently of the guest's memory limit.
type limitedLog struct{ text strings.Builder }

func (l *limitedLog) Write(data []byte) (int, error) {
	if left := 64*1024 - l.text.Len(); left > 0 {
		l.text.Write(data[:min(left, len(data))])
	}
	return len(data), nil
}
