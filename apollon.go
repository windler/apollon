package main

import (
	"flag"
	"html/template"
	"os"

	"github.com/windler/apollon/analyzer"
	cachegrind "github.com/windler/go-cachegrind"

	"gopkg.in/abiosoft/ishell.v2"
)

var ana analyzer.CachegrindAnalyzer
var outputTemplate string

func main() {
	head := `
     ___           ___           ___           ___       ___       ___           ___     
    /\  \         /\  \         /\  \         /\__\     /\__\     /\  \         /\__\    
   /::\  \       /::\  \       /::\  \       /:/  /    /:/  /    /::\  \       /::|  |   
  /:/\:\  \     /:/\:\  \     /:/\:\  \     /:/  /    /:/  /    /:/\:\  \     /:|:|  |   
 /::\~\:\  \   /::\~\:\  \   /:/  \:\  \   /:/  /    /:/  /    /:/  \:\  \   /:/|:|  |__ 
/:/\:\ \:\__\ /:/\:\ \:\__\ /:/__/ \:\__\ /:/__/    /:/__/    /:/__/ \:\__\ /:/ |:| /\__\
\/__\:\/:/  / \/__\:\/:/  / \:\  \ /:/  / \:\  \    \:\  \    \:\  \ /:/  / \/__|:|/:/  /
     \::/  /       \::/  /   \:\  /:/  /   \:\  \    \:\  \    \:\  /:/  /      |:/:/  / 
     /:/  /         \/__/     \:\/:/  /     \:\  \    \:\  \    \:\/:/  /       |::/  /  
    /:/  /                     \::/  /       \:\__\    \:\__\    \::/  /        /:/  /   
    \/__/                       \/__/         \/__/     \/__/     \/__/         \/__/    ... v 0.2
 `

	shell := ishell.New()
	shell.Println(head)

	initOutputTemplate()
	createAnalyzer(shell)
}

func initOutputTemplate() {
	outputTemplate = "({{.Row}}) {{.Measurement}} {{.Unit}} [{{.Percentage}}%]:\n"
	outputTemplate += "\t{{abbr .CallerFile 60}}:{{abbr .CallerFn 40}}\n"
	outputTemplate += "\t-> {{abbr .CalleeFile 60}}::{{abbr .CalleeFn 40}}\n"
	outputTemplate += "\t(line: {{.Line}}, times: {{.Occurences}})\n\n"
}

type callOutput struct {
	Row, Line, Occurences                      int
	CallerFn, CallerFile, CalleeFn, CalleeFile template.HTML
	Measurement, Percentage, Unit              string
}

func createAnalyzer(shell *ishell.Shell) {
	scheme := flag.String("scheme", "http", "neo4j scheme")
	host := flag.String("host", "localhost", "neo4j host")
	port := flag.Int("port", 7474, "neo4j port")
	username := flag.String("user", "neo4j", "neo4j user")
	pw := flag.String("password", "neo4j", "neo4j password")

	file := os.Args[1]

	pb := shell.ProgressBar()
	pb.Indeterminate(true)
	pb.Prefix("Creating database... ")
	pb.Start()

	if _, err := os.Stat(file); err != nil {
		panic(err.Error())
	}

	cg, err := cachegrind.Parse(file)
	if err != nil {
		panic(err.Error())
	}

	ana = &analyzer.Neo4jAnalyzer{
		Host:     *host,
		Password: *pw,
		Port:     *port,
		User:     *username,
		Scheme:   *scheme,
	}
	ana.Init(cg)

	pb.Suffix(" Done!")
	pb.Stop()
}
