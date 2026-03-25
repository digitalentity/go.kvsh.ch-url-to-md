package main

import (
	"fmt"
	"os"

	urltomd "go.kvsh.ch/url-to-md"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: example <url>")
		os.Exit(1)
	}

	article, err := urltomd.Convert(os.Args[1],
		urltomd.WithUserAgent("MyReader/1.0"),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("Title:   %s\n", article.Title)
	fmt.Printf("Byline:  %s\n", article.Byline)
	fmt.Printf("Excerpt: %s\n\n", article.Excerpt)
	fmt.Println(article.Content)
}
