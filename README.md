# esupload

This tool provides a dramatically simple interface for uploading JSON objects to ElasticSearch.

This is intended for the times you want to ingest a static data set into ElasticSearch to do visualization / search. Do not use this tool for long-term ingestion of data, as it only uses one index.

## Usage
```
Usage of esupload:
  -bulkSize int
        Number to documents to send at once. (default 1000)
  -dataSource string
        File to read items from, - for stdin. (default "-")
  -esAddress string
        Address of one ElasticSearch node. (default "http://localhost:9200")
  -idField string
        Field to pull the document id from. Set to '-' to do do create operations instead. (default "id")
  -indexName string
        Name of the index to put items into. (default "index")
```

Pipe JSON objects to the tool over stdin (or use `-dataSource` to read from a file).
```shell
cat data.json | esupload
```

## Limitations
* Only uploads to one index. This is not sufficient for long-term usage of continually ingesting data-sets (i.e. logs).
* The ID field *must* be a string. Other datatypes are not supported.
* Does not tolerate failures or perform retries.
* Does not understand how to pass credentials to ElasticSearch.