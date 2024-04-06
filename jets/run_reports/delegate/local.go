package delegate

import (
	"os"

	"github.com/xitongsys/parquet-go/source"
)

type LocalFile struct {
	FilePath string
	File     *os.File
}

func NewLocalFile(name string, fp *os.File) source.ParquetFile {
	return &LocalFile{
		FilePath: name,
		File: fp,
	}
}

func NewLocalFileWriter(name string) (source.ParquetFile, error) {
	return (&LocalFile{}).Create(name)
}

func NewLocalFileReader(name string) (source.ParquetFile, error) {
	return (&LocalFile{}).Open(name)
}

func (localFile *LocalFile) Create(name string) (source.ParquetFile, error) {
	file, err := os.Create(name)
	myFile := new(LocalFile)
	myFile.FilePath = name
	myFile.File = file
	return myFile, err
}

func (localFile *LocalFile) Open(name string) (source.ParquetFile, error) {
	var (
		err error
	)
	if name == "" {
		name = localFile.FilePath
	}

	myFile := new(LocalFile)
	myFile.FilePath = name
	myFile.File, err = os.Open(name)
	return myFile, err
}
func (localFile *LocalFile) Seek(offset int64, pos int) (int64, error) {
	return localFile.File.Seek(offset, pos)
}

func (localFile *LocalFile) Read(b []byte) (cnt int, err error) {
	var n int
	ln := len(b)
	for cnt < ln {
		n, err = localFile.File.Read(b[cnt:])
		cnt += n
		if err != nil {
			break
		}
	}
	return cnt, err
}

func (localFile *LocalFile) Write(b []byte) (n int, err error) {
	return localFile.File.Write(b)
}

func (localFile *LocalFile) Close() error {
	return localFile.File.Close()
}