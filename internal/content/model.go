package content

import "time"

type Entry struct {
	Title       string
	Description string
	ContentType string
	Date        time.Time
	LastMod     time.Time
	BlueskyURL  string
	RootURI     string
	RootCID     string
	PostCount   int
	Tags        []string
	Images      []Image
	Posts       []Post
	QuotedPosts []Quote
}

type Post struct {
	URI       string
	Text      string
	Markdown  string
	CreatedAt time.Time
}

type Image struct {
	FileName string
	Alt      string
	Source   string
	Width    int
	Height   int
}

type Quote struct {
	URL    string
	Author string
	Text   string
}
