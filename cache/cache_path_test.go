package cache

import "testing"

func TestDiskPathAcceptsSingleCharacterRelativePath(t *testing.T) {
	if _, err := diskPath("a"); err != nil {
		t.Fatalf("diskPath(\"a\") returned error: %v", err)
	}
}
