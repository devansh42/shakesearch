package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"index/suffixarray"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	searcher := Searcher{}
	err := searcher.Load("completeworks.txt")
	if err != nil {
		log.Fatal(err)
	}

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	http.HandleFunc("/search", handleSearch(searcher))

	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}

	fmt.Printf("Listening on port %s...", port)
	err = http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil {
		log.Fatal(err)
	}
}

type Searcher struct {
	CompleteWorks []byte
	SuffixArray   *suffixarray.Index
	Paragraphs    []int
}

func handleSearch(searcher Searcher) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		query, ok := r.URL.Query()["q"]
		if !ok || len(query[0]) < 1 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("missing search query in URL params"))
			return
		}
		results := searcher.Search(query[0])
		buf := &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		err := enc.Encode(results)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("encoding failure"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buf.Bytes())
	}
}

func (s *Searcher) Load(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Load: %w", err)
	}
	defer f.Close()
	loweredStream, originalStream := s.ToLower(f)
	dat, err := ioutil.ReadAll(loweredStream)
	if err != nil {
		return fmt.Errorf("Couldn't read lower case stream: %w", err)
	}
	s.SuffixArray = suffixarray.New(dat)

	s.CompleteWorks, err = ioutil.ReadAll(originalStream)
	if err != nil {
		return fmt.Errorf("Couldn't read actual file stream: %w", err)
	}

	return nil
}

// To lower will
func (s *Searcher) ToLower(reader io.Reader) (io.Reader, io.Reader) {
	bufReader := bufio.NewReader(reader)
	loweredReader := new(bytes.Buffer)
	originalReader := new(bytes.Buffer)
	s.Paragraphs = append(s.Paragraphs, 0) // Starting of the text
	nextParagraph := -1
	var pstring string
	i := 0
	var even = true
	for b, err := bufReader.ReadByte(); err == nil; b, err = bufReader.ReadByte() {
		originalReader.WriteByte(b)
		if b >= 65 && b <= 90 {
			b += 32
		} else { // Here we are recording all the paragraph breaks (if any)
			if b == '\r' {
				nextParagraph = i
				pstring = "\r"
			} else if b == '\n' && strings.HasPrefix(pstring, "\r") {
				pstring += string(b)
			} else {
				if nextParagraph != -1 {
					var x = i
					if even {
						x = nextParagraph
					}
					s.Paragraphs = append(s.Paragraphs, x)
					nextParagraph = -1
				}
			}
		}
		loweredReader.WriteByte(b)
		i++
	}
	return loweredReader, originalReader
}

func (s *Searcher) Search(query string) []string {
	idxs := s.SuffixArray.Lookup([]byte(strings.ToLower(query)), -1)
	results := []string{}
	bud := new(strings.Builder)
	l := len(query)
	for _, idx := range idxs {
		open, close := findEnclosingParagraph(s.Paragraphs, idx)
		bud.WriteString("<tr>============================================<pre>")
		bud.Write(s.CompleteWorks[open:idx])
		bud.WriteString("<b>")
		bud.Write(s.CompleteWorks[idx : idx+l])
		bud.WriteString("</b>")
		bud.Write(s.CompleteWorks[idx+l : close])
		bud.WriteString("</pre></tr>")
		results = append(results, bud.String())

	}
	return results
}

// Finds the enclosing paragraph, around a found match
func findEnclosingParagraph(ar []int, idx int) (int, int) {
	var i, open, close int
	for i = 0; i < len(ar) && ar[i] < idx; i++ {
	}
	if i < len(ar)-1 {
		if i > 0 {
			open = ar[i-1]

		}
		close = ar[i+1]
	}
	return open, close
}
