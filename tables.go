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
	"text/template"

	"github.com/dominikbraun/graph"
	"github.com/dominikbraun/graph/draw"
	humanize "github.com/dustin/go-humanize"
)

var filename = flag.String("f", "tables.csv", "Should contain a csv")
var outfilename = flag.String("o", "graph", "Output file for the visualization")
var outtype = flag.String("t", "graphviz", "Type of output")

func init() {
	flag.Parse()
}

type Graph interface {
	Ext() string
	AddNode(name string, weight int) error
	AddEdge(from, to, relationName string) error
	Draw(f io.Writer) error
}

var htmlTemplate = template.Must(template.ParseFiles("templ.html"))

type Node struct {
	Id         string         `json:"id"`
	Labels     []string       `json:"labels"`
	Properties map[string]any `json:"properties"`
	NodeRadius int64          `json:"nodeRadius"`
}

type Relationship struct {
	Id         string         `json:"id"`
	Type       string         `json:"type"`
	Source     string         `json:"source"`
	Target     string         `json:"target"`
	Labels     []string       `json:"labels"`
	Properties map[string]any `json:"properties"`
}

type d3 struct {
	Nodes         []*Node         `json:"nodes"`
	Relationships []*Relationship `json:"relationships"`
}

func newD3() *d3 {
	return &d3{}
}

func (g *d3) Ext() string {
	return ".html"
}

func (g *d3) AddNode(name string, weight int) error {
	rad := 2 * math.Log(float64(weight))
	g.Nodes = append(g.Nodes, &Node{
		Id:     name,
		Labels: []string{name},
		Properties: map[string]any{
			"size": humanize.IBytes(uint64(weight)),
		},
		NodeRadius: int64(rad),
	})
	return nil
}

func (g *d3) AddEdge(from, to, relationName string) error {
	g.Relationships = append(g.Relationships, &Relationship{
		Id:     fmt.Sprintf("%d", len(g.Relationships)),
		Type:   relationName,
		Source: from,
		Target: to,
		Labels: []string{relationName},
		Properties: map[string]any{
			"from": from,
			"to":   to,
		},
	})
	return nil
}

func (g *d3) Draw(f io.Writer) error {
	buf, err := json.Marshal(g)
	if err != nil {
		return err
	}
	return htmlTemplate.Execute(f, map[string]string{
		"json": string(buf),
	})
}

type graphViz struct {
	graph.Graph[string, string]
}

func newGV() *graphViz {
	return &graphViz{
		graph.New(graph.StringHash, graph.Directed()),
	}
}

func (g *graphViz) Ext() string {
	return ".gv"
}

func (g *graphViz) AddNode(name string, weight int) error {
	area := int64(math.Log10(float64(weight)))
	return g.AddVertex(name,
		graph.VertexWeight(weight),
		graph.VertexAttribute("comment", name),
		graph.VertexAttribute("shape", "circle"),
		graph.VertexAttribute("width", fmt.Sprintf("%d", area)),
		graph.VertexAttribute("height", fmt.Sprintf("%d", area)))
}

func (g *graphViz) AddEdge(from, to, relationName string) error {
	err := g.Graph.AddEdge(from, to, graph.EdgeAttribute("comment", relationName))
	if err == graph.ErrEdgeAlreadyExists {
		return nil
	}
	return err
}

func (g *graphViz) Draw(f io.Writer) error {
	return draw.DOT(g.Graph, f,
		draw.GraphAttribute("overlap", "false"),
		draw.GraphAttribute("splines", "true"),
		draw.GraphAttribute("fixedsize", "true"))
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

	var g Graph
	switch *outtype {
	case "graphviz":
		g = newGV()
	case "d3":
		g = newD3()
	}

	for _, record := range records {
		tableName, relationSize := record[0], record[2]
		rsize, err := strconv.Atoi(relationSize)
		if err != nil {
			log.Fatalf("Error converting relation_size to integer: %s, err: %v", relationSize, err)
		}
		log.Println("Adding vertex:", tableName, "with weight:", rsize)
		err = g.AddNode(tableName, rsize)
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
			err = g.AddEdge(tableName, fkTableName, fkCol)
			if err != nil {
				log.Fatalf("Error adding edge from: %s to: %s with fk col: %s, err: %v", tableName, fkTableName, fkCol, err)
			}
		}
	}

	f, err = os.Create(*outfilename + g.Ext())
	if err != nil {
		log.Fatalf("Error creating outfile: %v", err)
	}

	if err = g.Draw(f); err != nil {
		log.Fatalf("Error drawing graph: %v", err)
	}
}
