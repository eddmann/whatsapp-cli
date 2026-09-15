package whatsapp

import (
	"testing"

	"go.mau.fi/whatsmeow"
)

func TestClassifyDetectsCommonDocumentMimetypes(t *testing.T) {
	cases := []struct {
		path         string
		wantMedia    whatsmeow.MediaType
		wantMimetype string
	}{
		{"resume.pdf", whatsmeow.MediaDocument, "application/pdf"},
		{"notes.txt", whatsmeow.MediaDocument, "text/plain; charset=utf-8"},
		{"data.json", whatsmeow.MediaDocument, "application/json"},
		{"report.CSV", whatsmeow.MediaDocument, "text/csv; charset=utf-8"},
		{"photo.jpg", whatsmeow.MediaImage, "image/jpeg"},
		{"clip.mp4", whatsmeow.MediaVideo, "video/mp4"},
		{"unknownext.zzz", whatsmeow.MediaDocument, "application/octet-stream"},
		{"noextension", whatsmeow.MediaDocument, "application/octet-stream"},
	}

	for _, c := range cases {
		gotMedia, gotMimetype := classify(c.path)
		if gotMedia != c.wantMedia {
			t.Errorf("classify(%q) media = %v, want %v", c.path, gotMedia, c.wantMedia)
		}
		if gotMimetype != c.wantMimetype {
			t.Errorf("classify(%q) mimetype = %q, want %q", c.path, gotMimetype, c.wantMimetype)
		}
	}
}
