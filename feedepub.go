package main

import (
	"code.google.com/p/go-charset/charset"
	"encoding/xml"
	"fmt"
	"github.com/mjibson/goread/rss"
	"github.com/nikolawannabe/epub"

	"log"
	"net/http"
	"net/url"
	"errors"
	"github.com/nikolawannabe/epub/onix/codelist5"
)

const (
	xhtmlMediaType = "application/xhtml+xml"
)

type FeedEpub struct{}

// makeOpf creates an epub opf for the specified rss feed.
func (e FeedEpub) makeOpf(rssFeed rss.Rss) (epub.Opf, []epub.Chapter, error) {
	if len(rssFeed.Items) == 0 {
		return epub.Opf{}, nil, errors.New("No feed items to build")
	}
	ids := []epub.Identifier{
		epub.Identifier{Value: rssFeed.BaseLink(),
		IdentifierType: codelist5.URN,},}
	creator := epub.Creator{
		Name: rssFeed.Items[0].Author,
		Role: "aut",
	}
	metadata := epub.Metadata{
		Language: "en", // maybe, lol!
		Title:    rssFeed.Title,
		Creator:  creator,
		Date:     rssFeed.LastBuildDate,
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
		FullPath:   fmt.Sprintf("OEBPS/%s.opf", rssFeed.Title),
		MediaType:  "application/oebps-package+xml",
		Identifiers: ids,
		Metadata:   metadata,
		Manifest:   manifest,
	}

	opf := epub.Opf{
		RootFiles: []epub.OpfRootFile{opfRootFile},
	}
	return opf, chapters, nil
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
	opf, chapters, err := e.makeOpf(rssFeed)
	if err != nil {
		return nil, "", "Unable to generate OPF", err
	}

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
		log.Printf(errString + fmt.Sprint(": %v", err))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(errString))
		return
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
