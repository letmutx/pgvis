package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"

	"github.com/dominikbraun/graph"
	"github.com/dominikbraun/graph/draw"
)

var filename = flag.String("f", "tables.csv", "Should contain a csv")
var outfilename = flag.String("o", "graph.gv", "Output file for the visualization")

func init() {
	flag.Parse()
}

func main() {
	f, err := os.Open(*filename)
	if err != nil {
		log.Fatalf("Failed to open file: %s", *filename)
	}

	r := csv.NewReader(f)

	var next = func() []string {
		record, err := r.Read()
		switch {
		case err == io.EOF:
			return []string{}
		case err != nil:
			log.Fatalf("Error trying to read csv: %v", err)
		}
		return record
	}

	// skip header
	next()

	records := [][]string{}
	for {
		record := next()
		if len(record) == 0 {
			break
		}
		records = append(records, record)
	}

	g := graph.New(graph.StringHash, graph.Directed())

	for _, record := range records {
		tableName, relationSize := record[0], record[2]
		rsize, err := strconv.Atoi(relationSize)
		if err != nil {
			log.Fatalf("Error converting relation_size to integer: %s, err: %v", relationSize, err)
		}
		area := int64(math.Log10(float64(rsize)))
		log.Println("Adding vertex:", tableName, "with weight:", rsize, "with area:", area)
		err = g.AddVertex(tableName,
			graph.VertexWeight(rsize),
			graph.VertexAttribute("comment", tableName),
			graph.VertexAttribute("shape", "circle"),
			graph.VertexAttribute("width", fmt.Sprintf("%d", area)),
			graph.VertexAttribute("height", fmt.Sprintf("%d", area)))
		if err != nil {
			log.Fatalf("Error adding vertex: %s, err: %v", tableName, err)
		}
	}

	for _, record := range records {
		tableName, fkJsonBuf := record[0], record[1]
		var fks map[string]string
		err := json.Unmarshal([]byte(fkJsonBuf), &fks)
		if err != nil {
			log.Fatalf("Error deserializing fks json for table: %s, err: %v", tableName, err)
		}

		for fkCol, fkTableName := range fks {
			log.Println("Adding edge from:", tableName, "to:", fkTableName, "with fk col:", fkCol)
			err = g.AddEdge(tableName, fkTableName, graph.EdgeAttribute("comment", fkCol))
			if err == graph.ErrEdgeAlreadyExists {
				// TODO: check how to fix later
				continue
			}
			if err != nil {
				log.Fatalf("Error adding edge from: %s to: %s with fk col: %s, err: %v", tableName, fkTableName, fkCol, err)
			}
		}
	}

	f, err = os.Create(*outfilename)
	if err != nil {
		log.Fatalf("Error creating outfile: %v", err)
	}
	if err = draw.DOT(g, f, draw.GraphAttribute("overlap", "false"), draw.GraphAttribute("splines", "true"), draw.GraphAttribute("fixedsize", "true")); err != nil {
		log.Fatalf("Error drawing dot: %v", err)
	}
}
