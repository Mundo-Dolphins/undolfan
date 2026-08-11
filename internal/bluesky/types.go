package bluesky

import "time"

type FeedResponse struct {
	Cursor string     `json:"cursor,omitempty"`
	Feed   []FeedItem `json:"feed"`
}

type FeedItem struct {
	Post Post `json:"post"`
}

type ThreadResponse struct {
	Thread ThreadNode `json:"thread"`
}

type ThreadNode struct {
	Post    Post         `json:"post"`
	Replies []ThreadNode `json:"replies,omitempty"`
}

type Post struct {
	URI       string    `json:"uri"`
	CID       string    `json:"cid,omitempty"`
	Author    Author    `json:"author"`
	Record    Record    `json:"record"`
	Embed     *Embed    `json:"embed,omitempty"`
	IndexedAt time.Time `json:"indexedAt,omitempty"`
}

type Author struct {
	DID         string `json:"did"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName,omitempty"`
}

type Record struct {
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
	Facets    []Facet   `json:"facets,omitempty"`
	Reply     *ReplyRef `json:"reply,omitempty"`
}

type ReplyRef struct {
	Root   StrongRef `json:"root"`
	Parent StrongRef `json:"parent"`
}

type StrongRef struct {
	URI string `json:"uri"`
	CID string `json:"cid,omitempty"`
}

type Facet struct {
	Index    ByteSlice      `json:"index"`
	Features []FacetFeature `json:"features"`
}

type ByteSlice struct {
	ByteStart int `json:"byteStart"`
	ByteEnd   int `json:"byteEnd"`
}

type FacetFeature struct {
	Type string `json:"$type"`
	URI  string `json:"uri,omitempty"`
	DID  string `json:"did,omitempty"`
	Tag  string `json:"tag,omitempty"`
}

type Embed struct {
	Type   string       `json:"$type,omitempty"`
	Images []EmbedImage `json:"images,omitempty"`
	Record *EmbedRecord `json:"record,omitempty"`
}

type EmbedImage struct {
	Alt         string       `json:"alt,omitempty"`
	Thumb       string       `json:"thumb,omitempty"`
	Fullsize    string       `json:"fullsize,omitempty"`
	AspectRatio *AspectRatio `json:"aspectRatio,omitempty"`
}

type AspectRatio struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type EmbedRecord struct {
	Record *Post `json:"record,omitempty"`
}
