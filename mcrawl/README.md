This is a cmd for running a simple web crawler, limited to a single subdomain.

usage: `mcrawl [-j] [-c #] starting_URL`

    -crawls all same-domain links, beginning from `starting_url`
    -use the -j flag for json-formatted output
    -use the -c flag to set the level of concurrency to # of goroutines

