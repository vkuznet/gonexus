package gonexus

import "sync"

// Config holds the package-wide default settings that the Python API exposes
// through nxgetconfig()/nxsetconfig(). Not every Python setting has a Go
// equivalent (there is no lazy loading or memory-mapped-array budget in this
// port - see the README "Differences from Python" section) but the fields
// that matter for file I/O are preserved.
type Config struct {
	// Compression is the default HDF5 filter used for new datasets written
	// by Save/NXFile.WriteFile (e.g. "gzip", "" for none).
	Compression string
	// Encoding is the text encoding assumed for byte strings read from
	// HDF5 files. Go strings are UTF-8, so this mainly documents intent.
	Encoding string
	// Recursive controls whether NXFile.ReadFile eagerly reads every group
	// and field in the file (true) or only the immediate children of each
	// group, deferring the rest until first access (false). gonexus always
	// performs an eager, recursive read (Recursive is effectively true) -
	// see README for details - but the flag is kept for API compatibility
	// and future lazy-loading support.
	Recursive bool
	// MaxSize is the maximum number of elements read into memory
	// automatically when opening a file; larger fields must be read
	// explicitly with NXfield.ReadData(). 0 means "no limit".
	MaxSize int
}

var (
	configMu sync.RWMutex
	config   = Config{
		Compression: "gzip",
		Encoding:    "utf-8",
		Recursive:   true,
		MaxSize:     10000,
	}
)

// GetConfig returns a copy of the current package configuration.
// Equivalent to Python's nxgetconfig().
func GetConfig() Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return config
}

// SetConfig updates the package configuration. Only non-zero-value fields
// that the caller explicitly wants to change should be set; typical usage
// is to fetch GetConfig(), mutate the copy, and pass it back:
//
//	cfg := gonexus.GetConfig()
//	cfg.Compression = "lzf"
//	gonexus.SetConfig(cfg)
//
// Equivalent to Python's nxsetconfig(**kwargs).
func SetConfig(cfg Config) {
	configMu.Lock()
	defer configMu.Unlock()
	config = cfg
}

// SetCompression sets the default HDF5 compression filter for new datasets.
// Equivalent to Python's nxsetcompression().
func SetCompression(filter string) {
	configMu.Lock()
	defer configMu.Unlock()
	config.Compression = filter
}

// GetCompression returns the default HDF5 compression filter.
// Equivalent to Python's nxgetcompression().
func GetCompression() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.Compression
}
