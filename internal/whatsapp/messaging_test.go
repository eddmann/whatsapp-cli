package whatsapp

import (
	"testing"

	"go.mau.fi/whatsmeow"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		path      string
		mediaType whatsmeow.MediaType
		mime      string
	}{
		{"photo.jpg", whatsmeow.MediaImage, "image/jpeg"},
		{"clip.MP4", whatsmeow.MediaVideo, "video/mp4"},
		{"note.ogg", whatsmeow.MediaAudio, "audio/ogg; codecs=opus"},
		{"track.mp3", whatsmeow.MediaAudio, "audio/mpeg"},
		{"deck.pdf", whatsmeow.MediaDocument, "application/pdf"},
		{"report.docx", whatsmeow.MediaDocument, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"rows.csv", whatsmeow.MediaDocument, "text/csv"},
		{"archive.tar.gz", whatsmeow.MediaDocument, "application/octet-stream"},
		{"noextension", whatsmeow.MediaDocument, "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			mediaType, mime := classify(tt.path)
			if mediaType != tt.mediaType {
				t.Errorf("media type = %v, want %v", mediaType, tt.mediaType)
			}
			if mime != tt.mime {
				t.Errorf("mime = %q, want %q", mime, tt.mime)
			}
		})
	}
}
