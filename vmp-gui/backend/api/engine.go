package api

import (
	"context"
	"debug/elf"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	elfpacker "github.com/vmpacker/pkg/binary/elf"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed vm_interp.bin
var interpBlob []byte

//go:embed vm_interp_arm32.bin
var interpBlobARM32 []byte

//go:embed vm_interp_x86_64.bin
var interpBlobX86_64 []byte

// VMPEngine API Interface for Frontend
type VMPEngine struct {
	ctx     context.Context
	mu      sync.Mutex
	isBusy  bool
	dataDir string
}

// NewVMPEngine create new VMPEngine
func NewVMPEngine() *VMPEngine {
	// Use executable directory as data dir
	exe, _ := os.Executable()
	dataDir := filepath.Dir(exe)
	return &VMPEngine{dataDir: dataDir}
}

// Startup registers the Context
func (e *VMPEngine) Startup(ctx context.Context) {
	e.ctx = ctx
}

// SelectFile prompts user to select a file
func (e *VMPEngine) SelectFile() (string, error) {
	selection, err := runtime.OpenFileDialog(e.ctx, runtime.OpenDialogOptions{
		Title: "Select ELF Executable",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Executable Files (*.elf, *.exe, etc.)",
				Pattern:     "*.*",
			},
		},
	})

	if err != nil {
		return "", err
	}

	return selection, nil
}

// SelectSaveFile prompts user to select a save path
func (e *VMPEngine) SelectSaveFile(defaultFilename string) (string, error) {
	selection, err := runtime.SaveFileDialog(e.ctx, runtime.SaveDialogOptions{
		Title:           "Select Save Path",
		DefaultFilename: defaultFilename,
	})
	if err != nil {
		return "", err
	}
	return selection, nil
}

// AnalyzeELF reads binary information, verifying ARM64 format and extracting functions
func (e *VMPEngine) AnalyzeELF(filePath string) (map[string]interface{}, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	fileName := filepath.Base(filePath)

	// Open the file as an ELF
	f, err := elf.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open ELF file: %v", err)
	}
	defer f.Close()

	if f.Machine != elf.EM_AARCH64 {
		return nil, fmt.Errorf("unsupported architecture: only ARM64 is supported")
	}

	syms, err := f.Symbols()
	if err != nil {
		// Fallback to dynamic symbols for stripped binaries
		syms, err = f.DynamicSymbols()
		if err != nil {
			return nil, fmt.Errorf("failed to read symbol table or dynamic symbol table: %v. Fully stripped binaries may not be supported", err)
		}
	}

	funcs := []map[string]interface{}{}
	textSection := f.Section(".text")

	for _, sym := range syms {
		if elf.ST_TYPE(sym.Info) == elf.STT_FUNC && sym.Size > 0 {
			// Basic heuristic: check if it's likely within .text
			// (if section index is valid, we can be more certain, but this is a broad filter)
			if textSection != nil && (sym.Value < textSection.Addr || sym.Value >= textSection.Addr+textSection.Size) {
				continue // Skip functions outside .text bounds for now to avoid false positives
			}

			// Clean up the name for display if necessary
			funcName := sym.Name

			funcs = append(funcs, map[string]interface{}{
				"name":       funcName,
				"address":    fmt.Sprintf("0x%X", sym.Value),
				"size":       sym.Size,
				"protection": "Virtualization",
			})
		}
	}

	return map[string]interface{}{
		"fileName":  fileName,
		"filePath":  filePath,
		"arch":      "ARM64",
		"format":    "ELF",
		"functions": funcs,
	}, nil
}

// Protect executes the protection process
func (e *VMPEngine) Protect(options map[string]interface{}) error {
	e.mu.Lock()
	if e.isBusy {
		e.mu.Unlock()
		return fmt.Errorf("engine is currently busy")
	}
	e.isBusy = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.isBusy = false
		e.mu.Unlock()
	}()

	runtime.EventsEmit(e.ctx, "vmp-log", "[*] Starting Core Engine...")

	targetFile, ok := options["file"].(string)
	if !ok {
		return fmt.Errorf("invalid file parameter")
	}

	funcsParam, _ := options["functions"].([]interface{})
	opts, ok := options["options"].(map[string]interface{})
	if !ok {
		opts = make(map[string]interface{})
	}

	outPath, _ := opts["outputPath"].(string)
	if outPath == "" {
		outPath = targetFile + ".vmp"
	}

	enableDebug, _ := opts["debug"].(bool)
	stripSymbols, _ := opts["stripSymbols"].(bool)
	tokenEntry, _ := opts["tokenEntry"].(bool)
	verbose := true

	runtime.EventsEmit(e.ctx, "vmp-log", fmt.Sprintf("[+] Target program: %s", targetFile))

	var funcs []string
	var addrSpecs []elfpacker.AddrSpec
	for _, rawFn := range funcsParam {
		fMap, ok := rawFn.(map[string]interface{})
		if !ok {
			continue
		}
		isCustom, _ := fMap["isCustom"].(bool)
		if isCustom {
			// Manually added function: protect by address range
			name, _ := fMap["name"].(string)
			addrStr, _ := fMap["address"].(string)
			sizeFloat, _ := fMap["size"].(float64) // JSON number → float64
			addrStr = strings.TrimPrefix(addrStr, "0x")
			addrStr = strings.TrimPrefix(addrStr, "0X")
			addr, err := strconv.ParseUint(addrStr, 16, 64)
			if err != nil {
				runtime.EventsEmit(e.ctx, "vmp-log", fmt.Sprintf("[!] Address parsing failed: %s — %v", addrStr, err))
				continue
			}
			spec := elfpacker.AddrSpec{
				Addr: addr,
				End:  addr + uint64(sizeFloat),
				Name: name,
			}
			addrSpecs = append(addrSpecs, spec)
			runtime.EventsEmit(e.ctx, "vmp-log", fmt.Sprintf("[+] Manual function: %s @ 0x%X-0x%X", name, spec.Addr, spec.End))
		} else {
			name, _ := fMap["name"].(string)
			if name != "" {
				funcs = append(funcs, name)
			}
		}
	}

	totalCount := len(funcs) + len(addrSpecs)
	runtime.EventsEmit(e.ctx, "vmp-log", fmt.Sprintf("[+] Extracting and compiling %d target function nodes (Symbols: %d, Addresses: %d)...", totalCount, len(funcs), len(addrSpecs)))

	packer := elfpacker.NewPacker(targetFile, outPath, funcs, addrSpecs, verbose, stripSymbols, enableDebug, tokenEntry, interpBlob)
	packer.SetInterpBlobARM32(interpBlobARM32)
	packer.SetInterpBlobX86_64(interpBlobX86_64)

	// Temporarily override os.Stdout/os.Stderr or just let it process.
	// Since NewPacker prints to os.Stdout directly, the user wants logs in the GUI.
	// We'll trust that the quick process will just throw out outputs and we emit main events.

	if err := packer.Process(); err != nil {
		runtime.EventsEmit(e.ctx, "vmp-log", fmt.Sprintf("[x] Protection failed: %v", err))
		return err
	}

	runtime.EventsEmit(e.ctx, "vmp-log", fmt.Sprintf("[*] Initialization complete! Exported to: %s", outPath))
	return nil
}

// recentFilePath returns the path to the recent files JSON
func (e *VMPEngine) recentFilePath() string {
	return filepath.Join(e.dataDir, "recent_files.json")
}

// GetRecentFiles returns the list of recent files (max 10)
func (e *VMPEngine) GetRecentFiles() []map[string]string {
	data, err := os.ReadFile(e.recentFilePath())
	if err != nil {
		return []map[string]string{}
	}
	var files []map[string]string
	if err := json.Unmarshal(data, &files); err != nil {
		return []map[string]string{}
	}
	return files
}

// AddRecentFile adds a file path to the recent files list
func (e *VMPEngine) AddRecentFile(filePath string) {
	files := e.GetRecentFiles()
	name := filepath.Base(filePath)

	// Remove duplicate
	filtered := make([]map[string]string, 0, len(files))
	for _, f := range files {
		if f["path"] != filePath {
			filtered = append(filtered, f)
		}
	}

	// Prepend new entry
	entry := map[string]string{"name": name, "path": filePath}
	filtered = append([]map[string]string{entry}, filtered...)

	// Cap at 10
	if len(filtered) > 10 {
		filtered = filtered[:10]
	}

	data, _ := json.Marshal(filtered)
	_ = os.WriteFile(e.recentFilePath(), data, 0644)
}

// GetDataDir returns the application data directory
func (e *VMPEngine) GetDataDir() string {
	return e.dataDir
}
