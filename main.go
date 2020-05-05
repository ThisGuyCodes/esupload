package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
)

type bulkOp struct {
	Create *op                    `json:"create,omitempty"`
	Index  *op                    `json:"index,omitempty"`
	Update *op                    `json:"update,omitempty"`
	Delete *op                    `json:"delete,omitempty"`
	Data   map[string]interface{} `json:"-"`
}

type op struct {
	ID    string `json:"_id,omitempty"`
	Index string `json:"_index,omitempty"`
}

func makeOps(ingest io.Reader) <-chan bulkOp {
	opChan := make(chan bulkOp)
	go func() {
		defer close(opChan)
		dec := json.NewDecoder(ingest)
		for {
			item := map[string]interface{}{}
			err := dec.Decode(&item)
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Fatalf("Error decoding item: %v", err)
			}

			newOp := bulkOp{
				Data: item,
			}

			if *idField != "-" {
				id, ok := item[*idField].(string)
				if !ok {
					log.Fatalf("Provided id field %q is not a string or doesn't exist", *idField)
				}

				newOp.Index = &op{
					ID: id,
				}
			} else {
				newOp.Create = &op{}
			}
			opChan <- newOp
		}
	}()
	return opChan
}

type encoder interface {
	Encode(interface{}) error
}

func writeOps(enc encoder, source <-chan bulkOp, count int) error {
	for i := 0; i < count; i++ {
		op, ok := <-source
		if !ok {
			return io.EOF
		}

		err := enc.Encode(op)
		if err != nil {
			return fmt.Errorf("Could not encode the op: %w", err)
		}
		err = enc.Encode(op.Data)
		if err != nil {
			return fmt.Errorf("Could not encode the op data: %w", err)
		}
	}
	return nil
}

var (
	esAddress  = flag.String("esAddress", "http://localhost:9200", "Address of one ElasticSearch node.")
	bulkSize   = flag.Int("bulkSize", 1000, "Number to documents to send at once.")
	idField    = flag.String("idField", "id", "Field to pull the document id from. Set to '-' to do do create operations instead.")
	indexName  = flag.String("indexName", "index", "Name of the index to put items into.")
	dataSource = flag.String("dataSource", "-", "File to read items from, - for stdin.")
)

func main() {
	flag.Parse()

	cfg := elasticsearch.Config{
		Addresses: []string{
			*esAddress,
		},
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   10,
			ResponseHeaderTimeout: time.Second,
			DialContext:           (&net.Dialer{Timeout: time.Second}).DialContext,
		},
		EnableRetryOnTimeout: true,
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error creating client: %v", err)
	}
	_, err = client.Info()
	if err != nil {
		log.Fatalf("Error pinging elasticsearch: %v", err)
	}

	err = client.DiscoverNodes()
	if err != nil {
		log.Fatalf("Error discovering nodes: %v", err)
	}

	var dataFile io.ReadCloser
	if *dataSource == "-" {
		dataFile = os.Stdin
	} else {
		dataFile, err = os.Open(*dataSource)
		if err != nil {
			log.Fatalf("Could not open data file: %v", err)
		}
	}
	defer dataFile.Close()

	opSource := makeOps(dataFile)
	buf := &bytes.Buffer{}
	done := false
	for !done {
		buf.Reset()
		enc := json.NewEncoder(buf)
		err := writeOps(enc, opSource, *bulkSize)
		if err == io.EOF {
			done = true
		} else if err != nil {
			log.Print(err)
			break
		}
		res, err := client.Bulk(buf, client.Bulk.WithIndex(*indexName))
		if err != nil {
			log.Fatalf("Error doing bulk op: %v", err)
			break
		}
		if res.StatusCode != http.StatusOK {
			io.Copy(os.Stderr, res.Body)
			res.Body.Close()
			os.Exit(1)
		}

		res.Body.Close()
	}
}
