package jobs

import "testing"

func TestYouTubeURL(t *testing.T) {
	for _, value := range []string{"https://youtube.com/watch?v=a", "https://www.youtube.com/watch?v=a", "https://youtu.be/a"} {
		if !youtubeURL(value) {
			t.Errorf("expected valid URL: %s", value)
		}
	}
	for _, value := range []string{"youtube.com/watch?v=a", "https://example.com/youtube.com", "https://vimeo.com/a"} {
		if youtubeURL(value) {
			t.Errorf("expected invalid URL: %s", value)
		}
	}
}
