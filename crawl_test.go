package crawl_test

import (
	"crawl"
	"testing"
)

func TestCrawl(t *testing.T) {
	addr := "https://monzo.com"
	t.Log(crawl.Crawl(addr, map[string][]string{}))
}
