package crawl

import (
	"log"
	"net/http"
	"net/url"

	"golang.org/x/net/html"
)

func Crawl(addr string, visited map[string][]string) map[string][]string {

	log.Printf("crawling %s", addr)

	if visited == nil {
		visited = map[string][]string{}
	}

	// parse the url into URL
	u, err := url.Parse(addr)
	if err != nil {
		log.Fatal(err)
	}

	// get the html for that url
	res, err := http.Get(u.String())
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// scrape link from that url
	// TODO: See if this is bad to use the body here
	// if it's ok, close it explicitly
	doc, err := html.Parse(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	links := []string{}
	// TODO: Think about how/whether we should handle relative
	// links, and whether we want to control for <base>.
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

	// save the url and link to output
	visited[addr] = links

	// recursively crawl each valid link from the scrape links
	for _, l := range links {
		// TODO: If this URL is valid (will be crawled) we will end up
		// parsing it from a string again when we crawl it.
		// If possible, refactor to only parse URL strings once.
		u2, err := u.Parse(l)
		if err != nil {
			log.Fatal(err)
		}

		// Limit crawling to same subdomain (ie, the same URL host)
		if u2.Host != u.Host {
			continue
		}

		// Don't crawl previousy visited links
		if _, ok := visited[u2.String()]; ok {
			continue
		}

		visited = Crawl(u2.String(), visited)
	}

	return visited
}
