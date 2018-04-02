package main

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"strconv"

	"github.com/windler/cache-bro/analyzer"

	"gopkg.in/abiosoft/ishell.v2"
)

var ana *analyzer.CachegrindParser
var outputTemplate string
var currentInspection, previousInspection *analyzer.Call

func main() {
	head := `
  ___    __    ___  _   _  ____     ____  ____  _____ 
 / __)  /__\  / __)( )_( )( ___)___(  _ \(  _ \(  _  )
( (__  /(__)\( (__  ) _ (  )__)(___)) _ < )   / )(_)( 
 \___)(__)(__)\___)(_) (_)(____)   (____/(_)\_)(_____) ... v 0.1
 
 `

	shell := ishell.New()
	shell.Println(head)

	initOutputTemplate()
	createAnalyzer(shell)
	currentInspection = ana.GetFirst()

	shell.AddCmd(&ishell.Cmd{
		Name: "top",
		Help: "shows top [n] cpu or memory calls",
		Func: func(c *ishell.Context) {
			n := 10
			if len(c.Args) == 1 {
				n, _ = strconv.Atoi(c.Args[0])
			}

			choice := c.MultiChoice([]string{
				analyzer.Time,
				analyzer.Memory,
				"back",
			}, fmt.Sprintf("Show top %d", n))

			var kind, unit string
			switch choice {
			case 0:
				kind = analyzer.Time
				unit = analyzer.UnitTime
			case 1:
				kind = analyzer.Memory
				unit = analyzer.UnitMemory
			default:
				c.Println("going back ...")
				return
			}

			calls := ana.GetTop(n, kind)
			reference := ana.GetMainMeasurements(kind)

			for n := 0; n < len(calls); n++ {
				printCall(calls[n], n+1, kind, unit, reference, os.Stdout)
			}
			c.Println("")
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "inspect",
		Help: "navigate through the cachegrind file",
		Func: func(c *ishell.Context) {

			c.MultiChoice([]string{

				"back",
				"exit",
			}, "Filters:")
			c.Println("going back ...")
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "filters",
		Help: "modify output filters",
		Func: func(c *ishell.Context) {

			chooices := []string{
				currentInspection.CallerFn,
			}
			if previousInspection != nil {
				chooices = append(chooices, previousInspection.CallerFn)
				chooices = append(chooices, "back")
			}

			chooices = append(chooices, "exit")

			c.MultiChoice(chooices, "Navigate:")

		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "output",
		Help: "modify output format",
		Func: func(c *ishell.Context) {
			choice := c.MultiChoice([]string{
				"set format",
				"reset",
				"back",
			}, "Current format: "+outputTemplate)

			switch choice {
			case 0:
				c.ShowPrompt(false)
				defer c.ShowPrompt(true)

				c.Print("Output-Template: ")
				outputTemplate = c.ReadLine()
				c.Println("Successfully set to " + outputTemplate)
			case 1:
				initOutputTemplate()
				c.Println("Successfully reset template to " + outputTemplate)
			default:
				c.Println("going back ...")
			}
		},
	})

	shell.Run()
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

func printCall(call *analyzer.Call, n int, kind, unit string, reference float32, writer io.Writer) {
	tmpl, err := template.New("calls").Funcs(template.FuncMap{
		"abbr": func(s template.HTML, l int) template.HTML {
			if len(s) <= l {
				return s
			}
			return template.HTML("..." + string(s[len(s)-l:]))
		},
	}).Parse(outputTemplate)

	if err != nil {
		panic(err)
	}

	//TODO Move this to analyzer
	measurement := (*call).Time
	if kind == analyzer.Memory {
		measurement = (*call).Memory
	}
	err = tmpl.Execute(writer, callOutput{
		Row:         n,
		CalleeFn:    template.HTML((*call).CalleeFn),
		CalleeFile:  template.HTML((*call).CalleeFile),
		CallerFn:    template.HTML((*call).CallerFn),
		CallerFile:  template.HTML((*call).CallerFile),
		Measurement: fmt.Sprintf("%.3f", measurement),
		Unit:        unit,
		Percentage:  fmt.Sprintf("%.3f", (measurement / float32(reference) * 100)),
		Line:        (*call).Line,
		Occurences:  (*call).Occurences,
	})

	if err != nil {
		panic(err)
	}
}

func createAnalyzer(shell *ishell.Shell) {
	file := os.Args[1]

	pb := shell.ProgressBar()
	pb.Indeterminate(true)
	pb.Prefix("Initializing... ")
	pb.Start()

	ana, _ = analyzer.NewFrom(file)

	pb.Suffix(" Done!")
	pb.Stop()
}
