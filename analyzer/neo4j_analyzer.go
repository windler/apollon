package analyzer

import (
	"log"

	"github.com/jmcvetta/neoism"
	cachegrind "github.com/windler/go-cachegrind"
	neo4j "github.com/windler/go-neo4j-cypher"
)

type Neo4jAnalyzer struct {
	Password, User string
	Port           int64
	Host           string
	Scheme         string
	nodeCacheFn    map[string]cachegrind.Function
	nodeCacheFile  map[string]cachegrind.Function
	db             neo4j.CypherClient
}

type CypherResult struct {
	From []struct {
		Name string `json:"from"`
	} `json:"r"`
}

func (a *Neo4jAnalyzer) Init(c cachegrind.Cachegrind) {
	a.nodeCacheFn = map[string]cachegrind.Function{}
	a.nodeCacheFile = map[string]cachegrind.Function{}

	a.db = neo4j.NewHTTPCypherClient(a.Scheme, a.Host, a.Port, a.User, a.Password)

	a.runCypher(`MATCH (n:function) DETACH DELETE n`, nil)
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

	a.execBatch(a.getCreateNodesFnsBatch(functions))
	//a.execBatch((a.db, a.getCreateNodesFilesBatch( files))

	a.runCypher(`CREATE INDEX on :function(name)`, nil)
	a.runCypher(`CREATE INDEX on :file(name)`, nil)

	a.execBatch(a.getCreateEdgesCalledBatch(functions))
	//a.execBatch((a.db, a.getCreateEdgesBelongsToBatch(functions))

	a.runCypher(`CREATE INDEX on :called(time_sec)`, nil)
	a.runCypher(`CREATE INDEX on :called(memory_kB)`, nil)
}

func (a *Neo4jAnalyzer) execBatch(batch []*neo4j.CypherStatement) {
	res, err := a.db.ExecuteBatch(batch)
	if err != nil {
		log.Println(err.Error())
	}

	if len(res.Errors) > 0 {
		log.Println(res.Errors)
	}

}

func (a *Neo4jAnalyzer) runCypher(query string, args neo4j.CypherParameters) {
	res, err := a.db.Execute(&neo4j.CypherStatement{
		Statement:  query,
		Parameters: args,
	})

	if err != nil {
		panic(err.Error())
	}

	if len(res.Errors) > 0 {
		log.Println(res.Errors)
	}
}

func (a *Neo4jAnalyzer) getCypherResult(query string, args neo4j.CypherParameters) neo4j.ExecuteResult {
	cypher := &neo4j.CypherStatement{
		Statement:  query,
		Parameters: args,
	}

	res, err := a.db.Execute(cypher)

	if err != nil {
		panic(err.Error())
	}

	return res
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

func (a *Neo4jAnalyzer) getCreateNodesFnsBatch(fns []cachegrind.Function) []*neo4j.CypherStatement {
	batch := []*neo4j.CypherStatement{}

	for _, fn := range fns {
		cypher := &neo4j.CypherStatement{
			Statement: `CREATE (:function { name : {fn} })`,
			Parameters: neo4j.CypherParameters{
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

func (a *Neo4jAnalyzer) getCreateEdgesCalledBatch(fns []cachegrind.Function) []*neo4j.CypherStatement {
	batch := []*neo4j.CypherStatement{}
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

			cypher := &neo4j.CypherStatement{
				Statement: `MATCH (a:function { name : {aFn} }),(b:function { name : {bFn} }) 
							CREATE (a)-[:called {time_sec: {time}, memory_kB: {memory}, line: {line}}]->(b)`,
				Parameters: neo4j.CypherParameters{
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

type TopCall struct {
	From string `json:"from`
}

func (a *Neo4jAnalyzer) GetTopNPrefixCalls(n int, prefix string) interface{} {
	res := a.getCypherResult(`MATCH p = (s:function)-[c:called]->(e:function)
			WHERE s.name =~ {prefix}
			WITH collect({
				from: s.name,
				from_id: id(s),
				to: e.name,
				to_id: id(e),
				time_sec: c.time_sec,
				memory_kB: c.memory_kB,
				line: c.line
				})[..{n}] as r, c
			ORDER BY c.time_sec DESC
			RETURN r
	`, neo4j.CypherParameters{
		"prefix": prefix + ".*",
		"n":      n,
	})

	return res.Map("r", func(rowValue interface{}, meta neo4j.CypherQueryResultValueMeta) interface{} {
		row := rowValue.([]interface{})
		rowMp := row[0].(map[string]interface{})
		return TopCall{
			From: rowMp["from"].(string),
		}
	})
}
