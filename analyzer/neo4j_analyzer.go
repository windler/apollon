package analyzer

import (
	"fmt"
	"log"

	"github.com/jmcvetta/neoism"
	cachegrind "github.com/windler/go-cachegrind"
)

type Neo4jAnalyzer struct {
	Password, User string
	Port           int
	Host           string
	Scheme         string
	nodeCacheFn    map[string]cachegrind.Function
	nodeCacheFile  map[string]cachegrind.Function
}

func (a *Neo4jAnalyzer) Init(c cachegrind.Cachegrind) {
	a.nodeCacheFn = map[string]cachegrind.Function{}
	a.nodeCacheFile = map[string]cachegrind.Function{}

	url := fmt.Sprintf("%s://%s:%s@%s:%d/db/data", a.Scheme, a.User, a.Password, a.Host, a.Port)
	db, _ := neoism.Connect(url)

	a.runCypher(db, `MATCH (n:function) DETACH DELETE n`, nil)
	//a.runCypher(db, `MATCH (n:file) DETACH DELETE n`, nil)

	main := c.GetMainFunction()

	functions := []cachegrind.Function{
		c.GetMainFunction(),
	}
	functions = append(functions, a.getFunctions(main)...)

	/*files := []cachegrind.Function{
		c.GetMainFunction(),
	}
	files = append(files, a.getFiles(main)...)*/

	a.execBatch(db, a.getCreateNodesFnsBatch(db, functions))
	//a.execBatch(db, a.getCreateNodesFilesBatch(db, files))

	a.runCypher(db, `CREATE INDEX on :function(name)`, nil)
	a.runCypher(db, `CREATE INDEX on :file(name)`, nil)

	a.execBatch(db, a.getCreateEdgesCalledBatch(db, functions))
	//a.execBatch(db, a.getCreateEdgesBelongsToBatch(db, functions))

	a.runCypher(db, `CREATE INDEX on :called(time_sec)`, nil)
	a.runCypher(db, `CREATE INDEX on :called(memory_kB)`, nil)
}

func (a *Neo4jAnalyzer) execBatch(db *neoism.Database, batch []*neoism.CypherQuery) {
	for i := 0; i < len(batch); i += 1000 {
		end := i + 1000

		if end > len(batch) {
			end = len(batch)
		}

		if err := db.CypherBatch(batch[i:end]); err != nil {
			log.Println(err.Error())
		}
	}
}

func (a *Neo4jAnalyzer) runCypher(db *neoism.Database, query string, args map[string]interface{}) {
	if err := db.Cypher(&neoism.CypherQuery{
		Statement:  query,
		Parameters: args,
	}); err != nil {
		panic(err.Error())
	}
}

func (a *Neo4jAnalyzer) getFunctions(fn cachegrind.Function) []cachegrind.Function {
	nodes := []cachegrind.Function{}
	for _, call := range fn.GetCalls() {
		called := call.GetFunction()
		fnName := called.GetName()

		if _, found := a.nodeCacheFn[fnName]; !found {
			a.nodeCacheFn[fnName] = called

			nodes = append(nodes, called)
			nodes = append(nodes, a.getFunctions(called)...)
		}
	}

	return nodes
}

/*
func (a *Neo4jAnalyzer) getFiles(fn cachegrind.Function) []cachegrind.Function {
	nodes := []cachegrind.Function{}
	for _, call := range fn.GetCalls() {
		called := call.GetFunction()
		fileName := called.GetFile()

		if _, found := a.nodeCacheFn[fileName]; !found {
			a.nodeCacheFn[fileName] = called

			nodes = append(nodes, called)
			nodes = append(nodes, a.getFiles(called)...)
		}
	}

	return nodes
}
*/

func (a *Neo4jAnalyzer) getCreateNodesFnsBatch(db *neoism.Database, fns []cachegrind.Function) []*neoism.CypherQuery {
	batch := []*neoism.CypherQuery{}

	for _, fn := range fns {
		cypher := &neoism.CypherQuery{
			Statement: `CREATE (:function { name : {fn} })`,
			Parameters: neoism.Props{
				"fn": fn.GetName(),
			},
		}

		batch = append(batch, cypher)
	}

	return batch
}

/*
func (a *Neo4jAnalyzer) getCreateNodesFilesBatch(db *neoism.Database, files []cachegrind.Function) []*neoism.CypherQuery {
	batch := []*neoism.CypherQuery{}

	for _, fn := range files {
		cypher := &neoism.CypherQuery{
			Statement: `CREATE (:file { name : {file} })`,
			Parameters: neoism.Props{
				"file": fn.GetFile(),
			},
		}

		batch = append(batch, cypher)
	}

	return batch
}
*/

func (a *Neo4jAnalyzer) getCreateEdgesCalledBatch(db *neoism.Database, fns []cachegrind.Function) []*neoism.CypherQuery {
	batch := []*neoism.CypherQuery{}
	for _, fn := range fns {
		for _, called := range fn.GetCalls() {
			var time int64 = 0
			var mem int64 = 0

			measurements := called.GetMeasurements()

			if t, found := measurements[Time]; found {
				time = t
			}

			if m, found := measurements[Memory]; found {
				mem = m
			}

			cypher := &neoism.CypherQuery{
				Statement: `MATCH (a:function { name : {aFn} }),(b:function { name : {bFn} }) 
							CREATE (a)-[:called {time_sec: {time}, memory_kB: {memory}, line: {line}}]->(b)`,
				Parameters: neoism.Props{
					"aFn":    fn.GetName(),
					"bFn":    called.GetFunction().GetName(),
					"time":   float64(time) / 1000 / 1000,
					"memory": float64(mem) / 1000,
					"line":   called.GetLine(),
				},
			}

			batch = append(batch, cypher)
		}
	}

	return batch
}

func (a *Neo4jAnalyzer) getCreateEdgesBelongsToBatch(db *neoism.Database, fns []cachegrind.Function) []*neoism.CypherQuery {
	batch := []*neoism.CypherQuery{}
	for _, fn := range fns {
		cypher := &neoism.CypherQuery{
			Statement: `MATCH (a:function { name : {aFn} }),(b:file { name : {bFile} }) 
							CREATE (a)-[:belongs_to]->(b)`,
			Parameters: neoism.Props{
				"aFn":   fn.GetName(),
				"bFile": fn.GetFile(),
			},
		}

		batch = append(batch, cypher)

	}

	return batch
}
