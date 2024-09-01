package tarextract

// hat tip https://gist.github.com/indraniel/1a91458984179ab4cf80

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
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
			log.Fatalf("ExtractTarGz: Next() failed: %s", err.Error())
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(fmt.Sprintf("%s/%s",baseDir, header.Name), 0755); err != nil {
				log.Printf("ExtractTarGz: Mkdir() failed, folder probably already exist: %s", err.Error())
			}
		case tar.TypeReg:
			outFile, err := os.OpenFile(fmt.Sprintf("%s/%s",baseDir, header.Name), os.O_RDWR|os.O_CREATE, 0755)
			if err != nil {
				log.Fatalf("ExtractTarGz: OpenFile() failed: %s", err.Error())
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				log.Fatalf("ExtractTarGz: Copy() failed: %s", err.Error())
			}
		default:
			err := fmt.Errorf(
				"ExtractTarGz: uknown type: %v in %s",
				header.Typeflag,
				header.Name)
			log.Println(err)
			return err
		}
	}
	return nil
}
