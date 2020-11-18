package crawl

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestCrawl(t *testing.T) {
	want := []Result{
		{URL: "https://monzo.com", Links: []string{"/", "/bar"}},
		{URL: "https://monzo.com/", Links: []string{"/foo", "https://monzo.com/bar"}},
		{URL: "https://monzo.com/foo", Links: []string{"/", "bar", "/baz"}},
		{URL: "https://monzo.com/bar", Links: []string{"https://community.monzo.com", "bar"}},
		{URL: "https://monzo.com/baz", Links: []string{"https://facebook.com"}},
	}

	fetchMem := func(addr string) ([]string, error) {
		for _, r := range want {
			if r.URL != addr {
				continue
			}
			return r.Links, nil
		}
		return nil, fmt.Errorf("url (%s) not found", addr)
	}

	c := NewCrawler(25)

	// Override the default fetcher for this test
	c.fetch = fetchMem

	got, err := c.Crawl("https://monzo.com")

	if err != nil {
		t.Errorf("Crawl erred when not expected")
	}

	sortResults := cmpopts.SortSlices(func(i, j Result) bool {
		return i.URL < j.URL
	})
	sortStrings := cmpopts.SortSlices(func(i, j string) bool {
		return i < j
	})

	if diff := cmp.Diff(want, got, sortResults, sortStrings); diff != "" {
		t.Errorf("Crawl() mismatch (-want +got):\n%s", diff)
	}

}

func TestScrape(t *testing.T) {
	cases := []struct {
		name string
		body []byte
		want []string
	}{
		// TODO: See QA or HTML expert about good test cases.
		{
			name: "just anchor",
			body: []byte(`<a href="monzo.com/foo">bar</a>`),
			want: []string{"monzo.com/foo"},
		},
		{
			name: "just broken anchor",
			body: []byte(`<a href="/no-closing-tag"`),
			want: nil,
		},
		{
			name: "basic HTML doc",
			body: []byte(`<!DOCTYPE html>
<html>
<body>

<a href="/foo">to foo</a>
<a href="/bar">to bar</a>
<p>a paragraph.</p>

</body>
</html> 
			`),
			want: []string{"/foo", "/bar"},
		},
		{
			name: "HTML doc with nested anchor",
			body: []byte(`<!DOCTYPE html>
<html>
<body>

<a href="/foo"><a href="/bar">to bar</a>to foo</a>
<p>a paragraph.</p>

</body>
</html> 
			`),
			want: []string{"/foo", "/bar"},
		},
		{
			name: "HTML doc with broken anchors",
			body: []byte(`<!DOCTYPE html>
<html>
<body>

<a href="/foo"<a href="/bar">to bar</a>to foo</a>
<p>a paragraph.</p>

</body>
</html> 
			`),
			want: []string{"/foo"},
		},
	}

	for _, c := range cases {
		got, _ := scrape(c.body)
		if diff := cmp.Diff(c.want, got); diff != "" {
			t.Errorf("scrape() mismatch (-want +got):\n%s", diff)
		}

	}
}
