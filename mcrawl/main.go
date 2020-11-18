package main

import (
	"crawl"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
)

func main() {

	numFetchers := flag.Int("c", 25, "Number of concurrently operating HTTP fetchers")
	jsonOut := flag.Bool("j", false, "Return results as json formatted string")
	flag.Parse()

	if len(flag.Args()) < 1 {
		log.Fatalln("You must provide a URL to start the crawl")
	}

	u, err := url.Parse(flag.Arg(0))
	if err != nil {
		log.Fatalf("Invalid URL (%s): %s\n", flag.Arg(0), err)
	}

	results, err := crawl.NewCrawler(*numFetchers).Crawl(u.String())

	if err != nil {
		log.Fatalln(err)
	}

	if *jsonOut {
		j, err := json.Marshal(results)
		if err != nil {
			log.Printf("error marshalling results to json")
			// Let's return the non-json results in this case
		} else {
			fmt.Println(string(j))
			return
		}
	}
	for _, r := range results {
		fmt.Printf("%s, %s\n", r.URL, r.Links)
	}

}
