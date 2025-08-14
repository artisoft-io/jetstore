package compute_pipes

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestDetectCrAsEol01(t *testing.T) {
	rawData := "col1,col2,col3,col4\rrow01c1,row01c2,row01c3,row01c4\rrow02c1,row02c2,row02c3,row02c4"
	r := strings.NewReader(rawData)
	result, err := DetectCrAsEol(r, "none")
	if err != nil {
		t.Fatal(err)
	}
	if !result {
		t.Error("expecting true")
	}
}

func TestDetectCrAsEol02(t *testing.T) {
	rawData := "col1,col2,col3,col4\nrow01c1,row01c2,row01c3,row01c4\nrow02c1,row02c2,row02c3,row02c4"
	r := strings.NewReader(rawData)
	result, err := DetectCrAsEol(r, "none")
	if err != nil {
		t.Fatal(err)
	}
	if result {
		t.Error("expecting false")
	}
}

func TestDetectCrAsEol03(t *testing.T) {
	rawData := "\"col1\rCC\",col2,col3,col4\r\nrow01c1,row01c2,row01c3,row01c4\r\nrow02c1,row02c2,row02c3,row02c4"
	r := strings.NewReader(rawData)
	result, err := DetectCrAsEol(r, "none")
	if err != nil {
		t.Fatal(err)
	}
	if result {
		t.Error("expecting false")
	}
}

func genTestData() string {
	return "This is a test with Ç string\nThe second row is here"
}

func TestDetectEncoding01(t *testing.T) {
	// t.Errorf("data contains %d runes", len([]rune(genTestData())))
	// t.Errorf("data contains %d bytes", len([]byte(genTestData())))
	// t.Errorf("data as string of length %d", len(genTestData()))
	// Test using UTF-8
	// reader := strings.NewReader(genTestData())
	pin, pout := io.Pipe()
	go func() {
		writer, err := WrapWriterWithEncoder(pout, "UTF-8")
		if err != nil {
			t.Error(err)
		}
		w := bufio.NewWriter(writer)
		n, err := w.WriteString(genTestData())
		if err != nil {
			t.Error(err)
		}
		w.Flush()
		pout.Close()
		fmt.Printf("Write %d bytes\n", n)
	}()

	buf := make([]byte, 25000)
	n, err := pin.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	buf = buf[:n]
	encoding, err := DetectEncoding(buf)
	if err != nil {
		t.Fatal(err)
	}
	if encoding != "UTF-8" {
		t.Errorf("Expecting UTF-8, got %s", encoding)
	}
}

func TestDetectEncoding02(t *testing.T) {
	// t.Errorf("data contains %d runes", len([]rune(genTestData())))
	// t.Errorf("data contains %d bytes", len([]byte(genTestData())))
	// t.Errorf("data as string of length %d", len(genTestData()))
	// Test using UTF-8
	// reader := strings.NewReader(genTestData())
	pin, pout := io.Pipe()
	go func() {
		writer, err := WrapWriterWithEncoder(pout, "UTF-16LE")
		if err != nil {
			t.Error(err)
		}
		w := bufio.NewWriter(writer)
		n, err := w.WriteString(genTestData())
		if err != nil {
			t.Error(err)
		}
		w.Flush()
		pout.Close()
		fmt.Printf("Write %d bytes\n", n)
	}()

	buf := make([]byte, 25000)
	n, err := pin.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	buf = buf[:n]
	encoding, err := DetectEncoding(buf)
	if err != nil {
		t.Fatal(err)
	}
	if encoding != "UTF-16LE" {
		t.Errorf("Expecting UTF-16LE, got %s", encoding)
	}
}

func TestDetectEncoding03(t *testing.T) {
	// t.Errorf("data contains %d runes", len([]rune(genTestData())))
	// t.Errorf("data contains %d bytes", len([]byte(genTestData())))
	// t.Errorf("data as string of length %d", len(genTestData()))
	// Test using UTF-8
	// reader := strings.NewReader(genTestData())
	pin, pout := io.Pipe()
	go func() {
		writer, err := WrapWriterWithEncoder(pout, "UTF-16BE")
		if err != nil {
			t.Error(err)
		}
		w := bufio.NewWriter(writer)
		n, err := w.WriteString(genTestData())
		if err != nil {
			t.Error(err)
		}
		w.Flush()
		pout.Close()
		fmt.Printf("Write %d bytes\n", n)
	}()

	buf := make([]byte, 25000)
	n, err := pin.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	buf = buf[:n]
	encoding, err := DetectEncoding(buf)
	if err != nil {
		t.Fatal(err)
	}
	if encoding != "UTF-16LE" {
		t.Errorf("Expecting UTF-16LE, got %s", encoding)
	}
}

func TestDetectEncoding04(t *testing.T) {
	// t.Errorf("data contains %d runes", len([]rune(genTestData())))
	// t.Errorf("data contains %d bytes", len([]byte(genTestData())))
	// t.Errorf("data as string of length %d", len(genTestData()))
	// Test using UTF-8
	// reader := strings.NewReader(genTestData())
	pin, pout := io.Pipe()
	go func() {
		writer, err := WrapWriterWithEncoder(pout, "ISO-8859-1")
		if err != nil {
			t.Error(err)
		}
		w := bufio.NewWriter(writer)
		n, err := w.WriteString(genTestData())
		if err != nil {
			t.Error(err)
		}
		w.Flush()
		pout.Close()
		fmt.Printf("Write %d bytes\n", n)
	}()

	buf := make([]byte, 25000)
	n, err := pin.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	buf = buf[:n]
	encoding, err := DetectEncoding(buf)
	if err != nil {
		t.Fatal(err)
	}
	if encoding != "ISO-8859-1" {
		t.Errorf("Expecting ISO-8859-1, got %s", encoding)
	}
}

func TestDetectEncoding05(t *testing.T) {
	// t.Errorf("data contains %d runes", len([]rune(genTestData())))
	// t.Errorf("data contains %d bytes", len([]byte(genTestData())))
	// t.Errorf("data as string of length %d", len(genTestData()))
	// Test using UTF-8
	// reader := strings.NewReader(genTestData())
	pin, pout := io.Pipe()
	go func() {
		writer, err := WrapWriterWithEncoder(pout, "ISO-8859-2")
		if err != nil {
			t.Error(err)
		}
		w := bufio.NewWriter(writer)
		n, err := w.WriteString(genTestData())
		if err != nil {
			t.Error(err)
		}
		w.Flush()
		pout.Close()
		fmt.Printf("Write %d bytes\n", n)
	}()

	buf := make([]byte, 25000)
	n, err := pin.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	buf = buf[:n]
	encoding, err := DetectEncoding(buf)
	if err != nil {
		t.Fatal(err)
	}
	if encoding != "ISO-8859-1" {
		t.Errorf("Expecting ISO-8859-1, got %s", encoding)
	}
}
