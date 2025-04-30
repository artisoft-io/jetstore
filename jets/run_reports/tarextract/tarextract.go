package tarextract

// hat tip https://gist.github.com/indraniel/1a91458984179ab4cf80
// hat tip https://gist.github.com/mimoo/25fc9716e0f1353791f5908f94d6e726

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func ExtractTarGz(gzipStream io.Reader, baseDir string) error {
	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		log.Fatal("ExtractTarGz: NewReader failed")
	}

	tarReader := tar.NewReader(uncompressedStream)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("ExtractTarGz: Next() failed: %s", err.Error())
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(fmt.Sprintf("%s/%s", baseDir, header.Name), 0755); err != nil {
				log.Printf("ExtractTarGz: Mkdir() failed, folder probably already exist: %s", err.Error())
			}
		case tar.TypeReg:
			err = extractFile(fmt.Sprintf("%s/%s", baseDir, header.Name), tarReader)
			if err != nil {
				return err
			}
		default:
			err = fmt.Errorf(
				"ExtractTarGz: uknown type: %v in %s",
				header.Typeflag,
				header.Name)
			log.Println(err)
			return err
		}
	}
	return nil
}

func extractFile(localFileName string, tarReader *tar.Reader) error {
	outFile, err := os.OpenFile(localFileName, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("ExtractTarGz: OpenFile() failed: %v", err)
	}
	defer outFile.Close()
	_, err = io.Copy(outFile, tarReader)
	if err != nil {
		return fmt.Errorf("ExtractTarGz: Copy() failed: %v", err)
	}
	return nil
}

func CreateTarGz(basePath string, inputPaths []string, outputPath string) error {

	// zip the file, make sure it is compressed for faster speed
	var buf bytes.Buffer
	err := compress(basePath, inputPaths, &buf)
	if err != nil {
		return err
	}

	// write the compressed file to disk
	return os.WriteFile(outputPath, buf.Bytes(), os.ModePerm)
}

// inputPaths full path, they will be saved relative to basePath in the archive
func compress(basePath string, inputPaths []string, buf io.Writer) error {
	zr := gzip.NewWriter(buf)
	defer zr.Close()
	tw := tar.NewWriter(zr)
	defer tw.Close()
	var err error

	for _, path := range inputPaths {
		if strings.HasSuffix(path, "/") {
			err = addFolder(basePath, path[:len(path)-1], tw)
		} else {
			err = addFile(basePath, path, tw)
		}
		if err != nil {
			log.Println(err)
			return err
		}
	}
	return nil
}

func addFolder(basePath, src string, tw *tar.Writer) error {

	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(basePath, file)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			defer data.Close()

			_, err = io.Copy(tw, data)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func addFile(basePath, src string, tw *tar.Writer) error {
	data, err := os.Open(src)
	if err != nil {
		return err
	}
	defer data.Close()

	fi, err := data.Stat()
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(fi, src)
	if err != nil {
		return err
	}

	relPath, err := filepath.Rel(basePath, src)
	if err != nil {
		return err
	}

	header.Name = filepath.ToSlash(relPath)
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	// copy the file
	_, err = io.Copy(tw, data)
	return err
}
