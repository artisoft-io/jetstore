package compute_pipes

import (
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
)

func TestArrowDates(t *testing.T) {
	// Test that date32 and date64 types are correctly converted to string
	fieldInfo := &FieldInfo{
		Name: "date32_field",
		Type: arrow.PrimitiveTypes.Date32.Name(),
	}
	v, err := ConvertToSchemaV2("2021-10-20", fieldInfo)
	if err != nil {
		t.Fatalf("Error converting date32: %v", err)
	}
	value, ok := v.(arrow.Date32)
	if !ok {
		t.Fatalf("Expected arrow.Date32, got %T", v)
	}
	if value.FormattedString() != "2021-10-20" {
		t.Errorf("Expected '2021-10-20', got '%s'", value.FormattedString())
	}

	fieldInfo.Type = arrow.PrimitiveTypes.Date64.Name()
	v, err = ConvertToSchemaV2("2021-10-20", fieldInfo)
	if err != nil {
		t.Fatalf("Error converting date64: %v", err)
	}
	value64, ok := v.(arrow.Date64)
	if !ok {
		t.Fatalf("Expected arrow.Date64, got %T", v)
	}
	if value64.FormattedString() != "2021-10-20" {
		t.Errorf("Expected '2021-10-20', got '%s'", value64.FormattedString())
	}
}