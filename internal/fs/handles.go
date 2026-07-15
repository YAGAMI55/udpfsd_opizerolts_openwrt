package fs

import (
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/pcm720/udpfsd/internal/fs/interfaces"
	"github.com/pcm720/udpfsd/udpfs"
)

// handle is either a file or a directory.
type handle interface {
	io.Closer
}

type blockDeviceHandle struct {
	*fileHandle
	totalSectorCount int64
}

type fileHandle struct {
	obj      interfaces.FileObject
	Name     func() string
	Read     func(p []byte) (n int, err error)
	Write    func(p []byte) (n int, err error)
	Seek     func(offset int64, whence int) (int64, error)
	Stat     func() (os.FileInfo, error)
	closeFn  func() error
	wr       writeState
	sync.Mutex
	readOnly bool
}

type writeState struct {
	dataBuffer     []byte
	sectorNumber   int64
	sectorCount    uint16
	totalChunks    uint16
	receivedChunks uint16
	blockWrite     bool
}

func (f *fileHandle) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}

type dirHandle struct {
	dirPath string
	entries []os.DirEntry
	index   int
	sync.Mutex
}

func (d *dirHandle) Close() error {
	return nil
}

func (s *Backend) allocHandle(h handle) int32 {
	s.Lock()
	defer s.Unlock()

	// Try to find a free handle first
	for i, handle := range s.handles {
		if handle == nil {
			s.handles[i] = h
			s.lastUsed[i] = time.Now()
			return int32(i + 1) // Handle 0 is reserved for block device
		}
	}

	// Если свободных нет — возвращаем ошибку EMFILE.
	// Вытеснение (eviction) ОТКЛЮЧЕНО, чтобы не убивать дескрипторы, используемые PS2.
	log.Printf("fs: no free handles left")
	return -udpfs.EMFILE
}

func (s *Backend) freeHandle(handle int32) bool {
	if handle == udpfs.BlockDeviceHandle {
		return true
	}

	s.Lock()
	defer s.Unlock()

	// Handle 0 is reserved for block device, external handles start with 1
	idx := int(handle - 1)

	// Проверяем, что индекс валидный
	if idx < 0 || idx >= len(s.handles) {
		log.Printf("fs: invalid handle %d for close", handle)
		return false
	}

	if h := s.handles[idx]; h != nil {
		h.Close()
		s.handles[idx] = nil
		return true
	}

	// Handle уже был закрыт
	log.Printf("fs: attempt to close already closed handle %d", handle)
	return false
}

func (s *Backend) getFile(handle int32) *fileHandle {
	if handle < 0 {
		return nil
	}
	if handle == udpfs.BlockDeviceHandle {
		if s.bdHandle == nil {
			return nil
		}
		return s.bdHandle.fileHandle
	}

	s.Lock()
	defer s.Unlock()

	// Handle 0 is reserved for block device, external handles start with 1
	idx := int(handle - 1)

	// Проверяем, что индекс валидный
	if idx < 0 || idx >= len(s.handles) {
		log.Printf("fs: invalid handle %d for getFile", handle)
		return nil
	}

	h := s.handles[idx]
	if h == nil {
		return nil
	}

	fh, ok := h.(*fileHandle)
	if !ok {
		return nil
	}

	s.lastUsed[idx] = time.Now()
	return fh
}

func (s *Backend) getFileByPath(hostPath string) *fileHandle {
	s.Lock()
	defer s.Unlock()

	for idx, f := range s.handles {
		if fh, ok := f.(*fileHandle); ok {
			if fh.Name() == hostPath {
				s.lastUsed[idx] = time.Now()
				return fh
			}
		}
	}
	return nil
}

// getFileState returns whether the file at hostPath is currently open, and if so whether for writing.
func (s *Backend) getFileState(hostPath string) (open bool, readOnly bool) {
	s.Lock()
	defer s.Unlock()

	for _, f := range s.handles {
		if fh, ok := f.(*fileHandle); ok && fh.Name() == hostPath {
			return true, fh.readOnly
		}
	}
	return false, true
}

func (s *Backend) getDir(handle int32) *dirHandle {
	s.Lock()
	defer s.Unlock()

	// Handle 0 is reserved for block device, external handles start with 1
	idx := int(handle - 1)

	// Проверяем, что индекс валидный
	if idx < 0 || idx >= len(s.handles) {
		log.Printf("fs: invalid handle %d for getDir", handle)
		return nil
	}

	h := s.handles[idx]
	if h == nil {
		return nil
	}

	dh, ok := h.(*dirHandle)
	if !ok {
		return nil
	}

	s.lastUsed[idx] = time.Now()
	return dh
}

func (s *Backend) newFileHandle(f interfaces.FileObject, readOnly bool) handle {
	return &fileHandle{
		obj:      f,
		Read:     f.Read,
		Write:    f.Write,
		Seek:     f.Seek,
		Name:     f.Name,
		Stat:     f.Stat,
		closeFn:  f.Close,
		readOnly: readOnly,
	}
}
