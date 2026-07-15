package fs

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pcm720/udpfsd/internal/fs/compression"
	"github.com/pcm720/udpfsd/udpfs"
)

func (s *Backend) Open(path string, flag int, isDir bool) (handle int32, stat udpfs.StatInfo, err error) {
	// Get full path
	fullPath, ok := s.resolvePath(path)
	if !ok {
		return 0, udpfs.StatInfo{}, os.ErrNotExist
	}

	open, fileReadOnly := s.getFileState(fullPath)
	wantWrite := (flag & (os.O_WRONLY | os.O_RDWR)) != 0
	openForWriting := open && !fileReadOnly
	if wantWrite && (s.readOnly || open) {
		log.Printf("fs: refusing to open %s for writing: file already opened or FS is read-only", fullPath)
		return 0, udpfs.StatInfo{}, os.ErrPermission
	}
	if !wantWrite && openForWriting {
		log.Printf("fs: refusing to open %s for reading: file already opened for writing", fullPath)
		return 0, udpfs.StatInfo{}, os.ErrPermission
	}

	//
	// Handle directory path
	//
	if isDir {
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return 0, udpfs.StatInfo{}, err
		}
		handle := s.allocHandle(&dirHandle{entries: entries, dirPath: fullPath})
		if handle < 0 {
			return handle, udpfs.StatInfo{}, nil
		}
		st, _ := os.Stat(fullPath)
		return handle, udpfs.StatInfoFromFile(st), nil
	}

	//
	// Handle files
	//

	// Check if file exists
	exists := fullPath != "" && s.pathExists(fullPath)

	if s.enableCompression {
		if !exists {
			// If requested file doesn't exist and decompression is enabled,
			// check if any of supported compressed ISO types are present
			if strings.ToLower(filepath.Ext(fullPath)) == ".iso" {
				// If path has .iso extension, trim it and probe every supported extension
				compressedPath := strings.TrimSuffix(fullPath, filepath.Ext(fullPath))
				if s.pathExists(compressedPath) {
					// Handle compressed image
					log.Printf("fs: decompressing %s\n", compressedPath)
					wrapper := compression.Open(compressedPath, s.compressionCacheSize)
					if wrapper != nil {
						st, _ := wrapper.Stat()
						handle := s.allocHandle(s.newFileHandle(wrapper, true))
						if handle < 0 {
							wrapper.Close()
							return handle, udpfs.StatInfo{}, nil
						}
						return handle, udpfs.StatInfoFromFile(st), nil
					}
				}
			}
		}
	}

	// Handle plain file
	if !exists && ((flag & os.O_CREATE) == 0) {
		// If file doesn't exist and O_CREATE is not set, fail
		return 0, udpfs.StatInfo{}, os.ErrNotExist
	}
	if (flag & os.O_CREATE) != 0 {
		// Ensure directory structure exists
		dir := filepath.Dir(fullPath)
		os.MkdirAll(dir, 0755)
	}

	f, err := os.OpenFile(fullPath, flag, 0644)
	if err != nil {
		log.Printf("fs: failed to open file %s with flag 0x%x: %v\n", fullPath, flag, err)
		return 0, udpfs.StatInfo{}, err
	}
	handle = s.allocHandle(s.newFileHandle(f, (flag&(os.O_RDWR|os.O_WRONLY) == 0)))
	if handle < 0 {
		f.Close()
		return handle, udpfs.StatInfo{}, nil
	}
	st, _ := f.Stat()
	return handle, udpfs.StatInfoFromFile(st), nil
}

func (s *Backend) Close(handle int32) error {
	if ok := s.freeHandle(handle); !ok {
		return os.ErrInvalid
	}
	return nil
}

// Read reads data from the file associated with the given handle.
// It guarantees that it will read exactly 'size' bytes unless an error occurs
// or EOF is reached. If EOF is reached after reading some bytes, it returns
// the partial read without an error (client will handle the short read).
// If EOF is reached immediately (0 bytes read), it returns io.EOF.
func (s *Backend) Read(handle int32, size uint32, readBuffer []byte) (int32, []byte, error) {
	f := s.getFile(handle)
	if f == nil {
		return 0, nil, os.ErrInvalid
	}
	f.Lock()
	defer f.Unlock()

	// Ensure the buffer is large enough
	if len(readBuffer) < int(size) {
		return 0, nil, fmt.Errorf("buffer too small: %d < %d", len(readBuffer), size)
	}

	totalRead := 0
	for totalRead < int(size) {
		n, err := f.Read(readBuffer[totalRead:size])
		totalRead += n

		if err != nil {
			if err == io.EOF {
				// If we've read something, return it (client will handle short read)
				if totalRead > 0 {
					break
				}
				// Otherwise, this is a real EOF condition
				return 0, nil, io.EOF
			}
			// Any other error is fatal
			log.Printf("fs: failed to read file %s: %v", f.Name(), err)
			return 0, nil, err
		}

		// If n == 0 and no error, this is unexpected; break to avoid infinite loop
		if n == 0 {
			break
		}
	}

	return int32(totalRead), readBuffer[:totalRead], nil
}

func (s *Backend) WriteStart(handle int32) error {
	f := s.getFile(handle)
	if f == nil {
		return os.ErrInvalid
	}
	f.Lock()
	defer f.Unlock()

	if f.readOnly {
		return os.ErrPermission
	}
	f.wr.blockWrite = false
	f.wr.dataBuffer = []byte{}
	f.wr.totalChunks = 0
	f.wr.receivedChunks = 0
	return nil
}

func (s *Backend) WriteChunk(handle int32, chunkNr, chunkSize, totalChunks uint16, chunk []byte) (done bool, err error) {
	f := s.getFile(handle)
	if f == nil {
		return false, os.ErrInvalid
	}
	f.Lock()
	defer f.Unlock()

	if chunkNr != f.wr.receivedChunks {
		return false, nil
	}
	f.wr.dataBuffer = append(f.wr.dataBuffer, chunk...)
	f.wr.totalChunks = totalChunks
	f.wr.receivedChunks++
	return f.wr.receivedChunks >= f.wr.totalChunks, nil
}

// CompleteWrite completes the current write (regular or block). Returns bytes
// written for regular, 0 for block on success.
func (s *Backend) CompleteWrite(handle int32) (n int32, err error) {
	f := s.getFile(handle)
	if f == nil {
		return 0, os.ErrInvalid
	}
	f.Lock()
	defer f.Unlock()

	sectorSize := s.sectorSize
	if handle != udpfs.BlockDeviceHandle {
		sectorSize = 512
	}

	data := append([]byte(nil), f.wr.dataBuffer...)

	if f.wr.blockWrite {
		expectedSize := int(f.wr.sectorCount) * sectorSize
		if len(data) > expectedSize {
			data = data[:expectedSize]
		}
		_, err := f.Seek(f.wr.sectorNumber*int64(sectorSize), 0)
		if err != nil {
			return 0, err
		}
		_, err = f.Write(data)
		if err != nil {
			log.Printf("fs: failed to write %s: %v", f.Name(), err)
			return 0, err
		}
		return 0, nil
	}

	if f.Write == nil {
		return -9, nil
	}
	written, err := f.Write(data)
	if err != nil {
		log.Printf("fs: failed to write %s: %v", f.Name(), err)
		return 0, err
	}
	return int32(written), nil
}

func (s *Backend) Lseek(handle int32, offset int64, whence int) (position int64, err error) {
	f := s.getFile(handle)
	if f == nil {
		return -1, nil
	}
	f.Lock()
	defer f.Unlock()
	return f.Seek(offset, whence)
}

func (s *Backend) Dread(handle int32) (ok bool, name string, stat udpfs.StatInfo, err error) {
	d := s.getDir(handle)
	if d == nil {
		return false, "", udpfs.StatInfo{}, os.ErrInvalid
	}
	d.Lock()
	defer d.Unlock()

	if d.index >= len(d.entries) {
		return false, "", udpfs.StatInfo{}, nil
	}
	entry := d.entries[d.index]
	d.index++

	dirPath := d.dirPath
	info, err := entry.Info()
	if err != nil {
		return false, "", udpfs.StatInfo{}, nil
	}
	name = entry.Name()

	var fullPath string
	if dirPath != "" && dirPath != "/" {
		fullPath = filepath.Join(dirPath, name)
	} else {
		fullPath = filepath.Join("/", name)
	}

	// Check if this is a directory
	if entry.IsDir() {
		// Return directory info
		return true, name, udpfs.StatInfoFromFile(info), nil
	}

	// For files, check if we have a compressed version
	if s.enableCompression {
		// Check if this file is a compressed image
		if strings.ToLower(filepath.Ext(name)) == ".iso" {
			// Try to find compressed version
			baseName := strings.TrimSuffix(name, filepath.Ext(name))
			// Check for compressed file in the same directory
			compressedPath := filepath.Join(filepath.Dir(fullPath), baseName)
			if s.pathExists(compressedPath) {
				// It's a compressed file, get its info
				compInfo, err := os.Stat(compressedPath)
				if err == nil {
					return true, name, udpfs.StatInfoFromFile(compInfo), nil
				}
			}
		}
	}

	return true, name, udpfs.StatInfoFromFile(info), nil
}
