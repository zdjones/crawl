package crawl

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sort"

	"golang.org/x/net/html"
)

// scrape attempts to find all the links in the provided HTML document.
// Passing invalid HTML may result in an error, but may also return invalid
// results, depending on how the HTML parser interprets the input.
func scrape(body []byte) ([]string, error) {

	// Scrape the links from that url
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to parse body as HTML: %w", err)
	}

	var links []string
	// TODO: We should really check for a <base> element.
	// If present, we'll need a way to include that with the results.
	// Currently, resolving these hrefs is not handled by the scraper,
	// think about whether it should be.
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					links = append(links, a.Val)
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return links, nil
}

func getHTTP(addr string) ([]byte, error) {
	res, err := http.Get(addr)
	if err != nil {
		return nil, fmt.Errorf("getHTTP(%s) failed GET request: %w", addr, err)
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("getHTTP(%s) got bad HTTP reponse code (%d): %s", addr, res.StatusCode, res.Status)
	}
	defer res.Body.Close()
	return ioutil.ReadAll(res.Body)
}

func fetchHTTP(addr string) ([]string, error) {

	body, err := getHTTP(addr)
	if err != nil {
		return nil, fmt.Errorf("fetchHTTP(%s) get: %w", addr, err)
	}

	links, err := scrape(body)
	if err != nil {
		return nil, fmt.Errorf("fetchHTTP(%s) scrape: %w", addr, err)
	}

	return links, nil

}

// Result is the results from a single page/URL.
type Result struct {
	URL   string
	Links []string
	Err   error
}

// Crawler is our means of managing configuration for a crawl instance.
type Crawler struct {
	numFetchers int
	fetch       func(string) ([]string, error)
}

// NewCrawler creates a Crawler with the given configuration (currently
// this is just the number of concurrent fetchers to run). The crawler's
// fetcher is only configurable internally by this package, for testing
// purposes.
func NewCrawler(numFetchers int) Crawler {
	return Crawler{
		numFetchers: numFetchers,
		fetch:       fetchHTTP,
	}
}

// startFetcher is used to start a fetcher. This is intended to be used
// as a concurrent worker. It is not of much help otherwise.
func (c Crawler) startFetcher(urls <-chan string, out chan<- Result) {
	// Fetch urls from the channel until closed.
	for u := range urls {
		r := Result{URL: u}
		r.Links, r.Err = c.fetch(r.URL)
		out <- r
	}
}

// Crawl orchestrates the crawling of all same-subdomain links, beginning at
// the provided address/URL. 'addr' must be a valid formatted URL. 'numfetchers'
// determines the number of fetchers operating concurrently. Aim for numfetchers
// to be high enough that we do not spend too much time blocked on network IO,
// but low enough that we don't assault the receiving HTTP servers and/or
// overflow our own stack.
// The results will be returned sorted by URL.
func (c Crawler) Crawl(addr string) ([]Result, error) {

	root, err := url.Parse(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid starting URL %s: %w", addr, err)
	}

	tofetch := make(chan string)
	fetched := make(chan Result)

	// Start a fixed number of fetchers. This will help us limit our
	// footprint on the servers we crawl. It is also just prudent
	// to control our own outlay of resources.
	for i := 0; i < c.numFetchers; i++ {
		go c.startFetcher(tofetch, fetched)
	}

	// Work queue - URLs to be crawled.
	// Start crawling at the given URL
	work := []string{addr}

	// TODO: This could be map[string]struct{} to save a bit of space, but the semantics of bool is apt.
	visited := make(map[string]bool)

	// We need to keep track of whether there is any fetching in progress, in order to know
	// when we are actually finished.
	fetching := 0

	var results []Result
	for {
		// If we currently have no urls to fetch, we have to be sure we aren't sending
		// the empty next var to the fetchers. We can do this by using a nil channel variable.
		// This nil channel will block forever, so the select case sending on it will never
		// match. On any iteration where we do have urls/work to send, we can swap out this
		// channel with the actual fetchers channel, thus allowing the next url to be sent.
		var sendWork chan<- string
		var next string
		if len(work) > 0 {
			sendWork = tofetch
			next = work[0]
			// In case any duplicates slip through to the work queue, don't fetch the again.
			if visited[next] {
				work = work[1:]
				continue
			}
		} else if fetching == 0 {
			// The queue is empty and no fetching is on progress. We are done crawling.
			// Signal to the fetchers that we are finished with them.
			close(tofetch)
			break
		}

		select {
		// If we have a url to crawl and a fetcher is available, send the url to them.
		case sendWork <- next:
			visited[next] = true
			work = work[1:]
			fetching++
		// If we have no url to crawl or there are no fetchers available,
		// process results coming back from the fetchers. This will unblock
		// any fetchers blocked on sending results back.
		// TODO: Determine whether this processing is blocking fetchers. Fetching is
		// where we need the concurrency (due to network IO), so we want to
		// be sure that we aren't holding any of that back due to processing delays.
		case page := <-fetched:
			fetching--

			base, err := url.Parse(page.URL)
			if err != nil {
				log.Println(err)
				// Don't continue processing links from an unparseable URL.
				break
			}
			// Process each link found on this page.
			for _, l := range page.Links {

				// Resolve link
				// We need to resolve the links, they are still just raw href values.
				// TODO: Should really consider the possibility that the page
				// was using <base> tag to resolve links
				link, err := base.Parse(l)
				if err != nil {
					log.Println(err)
					// Don't further process this bad/unparseable link.
					continue
				}

				// Filter link
				// Clear the fragment and query for more accurate comparison.
				link.Fragment = ""
				link.RawQuery = ""
				l = link.String()

				// TODO: query requirements to see if results should
				// be resolved URLS or not.
				// If yes, use this: page.Links[i] = l

				// We only want to enqueue non-duplicate, same-host URLS
				if link.Host != root.Host {
					continue
				}
				if visited[l] {
					continue
				}
				work = append(work, l)
			}
			results = append(results, page)
		}

	}

	// Clean up the results.
	for _, res := range results {
		sort.Strings(res.Links)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].URL < results[j].URL
	})

	return results, nil
}
