package schema

import (
	"reflect"
	"testing"
)

func TestUniquefyHeaders_NoDuplicates(t *testing.T) {
	headers := []string{"id", "name", "email"}
	expected := []string{"id", "name", "email"}
	result, modified := uniquefyHeaders(headers)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
	if modified {
		t.Errorf("Expected no modifications, but headers were modified")
	}
}

func TestUniquefyHeaders_WithDuplicates(t *testing.T) {
	headers := []string{"id", "name", "name", "email", "id"}
	expected := []string{"id", "name", "name_2", "email", "id_2"}
	result, modified := uniquefyHeaders(headers)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
	if !modified {
		t.Errorf("Expected modifications, but headers were not modified")
	}
}

func TestUniquefyHeaders_AllDuplicates(t *testing.T) {
	headers := []string{"a", "a", "a"}
	expected := []string{"a", "a_2", "a_3"}
	result, modified := uniquefyHeaders(headers)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
	if !modified {
		t.Errorf("Expected modifications, but headers were not modified")
	}
}

func TestUniquefyHeaders_EmptySlice(t *testing.T) {
	headers := []string{}
	result, modified := uniquefyHeaders(headers)
	if len(result) != 0 {
		t.Errorf("Expected [], got %v", result)
	}
	if modified {
		t.Errorf("Expected no modifications, but headers were modified")
	}
}

func TestNewHeadersUniquefied(t *testing.T) {
	headers := []string{"foo", "bar", "foo"}
	h := NewHeadersUniquefied(headers)
	expectedUnique := []string{"foo", "bar", "foo_2"}
	if !reflect.DeepEqual(h.OriginalHeaders, headers) {
		t.Errorf("OriginalHeaders: expected %v, got %v", headers, h.OriginalHeaders)
	}
	if !reflect.DeepEqual(h.UniqueHeaders, expectedUnique) {
		t.Errorf("UniqueHeaders: expected %v, got %v", expectedUnique, h.UniqueHeaders)
	}
}
