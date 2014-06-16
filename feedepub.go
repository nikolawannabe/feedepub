package main

import (
	"code.google.com/p/go-charset/charset"
	"encoding/xml"
	"fmt"
	"github.com/mjibson/goread/rss"
	"log"
	"net/http"
	"github.com/nikolawannabe/epub"
	"net/url"
)

const (
	xhtmlMediaType = "application/xhtml+xml"
)

type FeedEpub struct {}

// makeOpf creates an epub opf for the specified rss feed.
func (e FeedEpub) makeOpf(rssFeed rss.Rss) (epub.Opf, []epub.Chapter) {
	id := epub.Identifier("this is a random id")
	creator := epub.Creator{
		Name: rssFeed.Items[0].Author, // TODO:we should make sure this exists first!
		Role: "aut",
	}
	metadata := epub.Metadata{
		Language: "en", // maybe, lol!
		Title: rssFeed.Title,
		Creator: creator,
		Date: rssFeed.LastBuildDate,
	}

	var manifestItems []epub.ManifestItem
	var chapters []epub.Chapter
	// TODO: This is a totally naive build of file names that may be non-unique.
	for i, item := range rssFeed.Items {
		var manifestItem epub.ManifestItem
		var chapter epub.Chapter
		//  TODO: in a better world this would be generated by sanitized filename
		manifestItem.Id = fmt.Sprintf("manifest_id_%d", i)
		filename := "chapters/" + item.Title + ".xhtml"
		manifestItem.Href = filename
		chapter.FileName = filename
		// TODO: we should fetch all the assets used in each rss item and put them in the
		// epub in an assets directory, then rewrite the paths to each asset.  At the
		// very least we should rewrite relative urls to actually point to the feed.
		chapter.Contents = item.Description
		manifestItem.MediaType = xhtmlMediaType
		manifestItems = append(manifestItems, manifestItem)
		chapters = append(chapters, chapter)
	}
	manifest := epub.Manifest{
		ManifestItems: manifestItems,
	}

	opfRootFile := epub.OpfRootFile{
		FullPath: fmt.Sprintf("XHTML/%s.opf", rssFeed.Title),
		MediaType: "application/oebps-package+xml",
		Identifier: id,
		Metadata: metadata,
		Manifest: manifest,
	}

	opf := epub.Opf{
		RootFiles: []epub.OpfRootFile{ opfRootFile},
	}
	return opf, chapters
}

// getFeed gets the feed for the specified rss url
func (e FeedEpub) getFeed(rssUrl string) (rss.Rss, string, error) {
	client := &http.Client{}
	rssFeed := rss.Rss{}

	resp, err := client.Get(rssUrl)
	if err != nil {
		return rssFeed, "Unable to fetch xml", err
	}
	defer resp.Body.Close()

	d := xml.NewDecoder(resp.Body)
	d.CharsetReader = charset.NewReader

	d.DefaultSpace = "DefaultSpace"

	if err := d.Decode(&rssFeed); err != nil {
		return rssFeed, "Unable to parse xml", err
	}

	return rssFeed, "", nil
}

// MakeBook fetches the feed and builds the epub out of it
func (e FeedEpub) MakeBook(rssUrl string) ([]byte, string, string, error) {
	rssFeed, errString, err := e.getFeed(rssUrl)
	if errString != "" {
		log.Printf("Unable to get feed: %s, %v", errString, err)
		return nil, "", "Server error", err
	}
	opf, chapters := e.makeOpf(rssFeed)
	var archive epub.EpubArchive
	bytes, err := archive.Build(rssFeed.Title, opf, chapters)
	if err != nil {
		log.Print("Unable to build epub")
		return nil, "", "Server Error", err
	}
	return bytes, rssFeed.Title, "", err

}

// downloadBook downloads the epub for the requested rss feed.
func downloadBook(w http.ResponseWriter, r *http.Request) {
	var feedpub FeedEpub
	rssString := r.URL.Query().Get("rssurl")
	log.Printf("URL: %s", rssString)
	// TODO: we should replace feed:// with http:// since that is reasonable here.
	rssUrl, err := url.Parse(rssString)
	if err != nil || !rssUrl.IsAbs() {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Parameters"))
		return
	}
	epubArchive, title, errString, err := feedpub.MakeBook(rssUrl.String())

	if errString != "" {
		w.Write([]byte(errString))
		log.Printf(errString + fmt.Sprint(": %v", err))
	}

	w.Header().Set("Content-Type", "application/epub+zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.epub\"",
			title))
	w.Write(epubArchive)
}

func main() {
	http.HandleFunc("/getepub", downloadBook)
	log.Fatal(http.ListenAndServe(":8081", nil))
}
