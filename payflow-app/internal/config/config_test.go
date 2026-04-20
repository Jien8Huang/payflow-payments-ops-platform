package config

import (
	"reflect"
	"testing"
)

func TestSplitComma(t *testing.T) {
	t.Parallel()
	got := SplitComma(" http://a:1 ,, https://b ")
	want := []string{"http://a:1", "https://b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitComma() = %#v, want %#v", got, want)
	}
	if len(SplitComma("")) != 0 {
		t.Fatal("empty input")
	}
}
