package compute_pipes

import (
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
