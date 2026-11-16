package dialogs

import "testing"

func TestFolderTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{"blank falls back to the default", "   ", defaultFolderTitle},
		{"empty falls back to the default", "", defaultFolderTitle},
		{"a given title is kept", "Link a folder", "Link a folder"},
		{"surrounding space is trimmed", "  Link a folder  ", "Link a folder"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := folderTitle(test.title); got != test.want {
				t.Fatalf("folderTitle(%q) = %q, want %q", test.title, got, test.want)
			}
		})
	}
}
