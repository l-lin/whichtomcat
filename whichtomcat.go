package main

import (
	"bufio"
	"bytes"
	ui "github.com/gizak/termui"
	tm "github.com/nsf/termbox-go"
	"github.com/spf13/cobra"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
)

func main() {
	var rootCmd = cobra.Command{
		Use:  "",
		Long: "",
		Run:  whichTomcat,
	}
	rootCmd.Execute()
}

func whichTomcat(cmd *cobra.Command, args []string) {
	processes := fetchProcesses()
	homes := fetchCatalinaBases(processes)

	tomcats := make([]*Tomcat, 0)
	for _, home := range homes {
		webapps := fetchWebapps(home)
		httpPort, httpsPort := fetchTomcatPort(home)
		tomcats = append(tomcats, &Tomcat{
			Home:      home,
			Webapps:   webapps,
			HttpPort:  httpPort,
			HttpsPort: httpsPort,
		})
	}
	displayTomcats(tomcats)
}

type Tomcat struct {
	Home      string
	Webapps   []string
	HttpPort  string
	HttpsPort string
}

func displayTomcats(tomcats []*Tomcat) {
	err := ui.Init()
	if err != nil {
		panic(err)
	}
	defer ui.Close()

	ui.UseTheme("helloworld")

	row := make([]ui.GridBufferer, 0)
	for _, tomcat := range tomcats {
		list := make([]string, len(tomcat.Webapps)+3)
		list[0] = "[HTTP]  " + tomcat.HttpPort
		list[1] = "[HTTPS] " + tomcat.HttpsPort
		list[2] = "------------------------------------------------"
		copy(list[3:], tomcat.Webapps)

		ls := ui.NewList()
		ls.Items = list
		ls.ItemFgColor = ui.ColorMagenta
		ls.Border.Label = tomcat.Home
		ls.Height = 15

		row = append(row, ls)
		if len(row) == 3 {
			ui.Body.AddRows(ui.NewRow(
				ui.NewCol(4, 0, row[0]),
				ui.NewCol(4, 0, row[1]),
				ui.NewCol(4, 0, row[2]),
			))
			row = make([]ui.GridBufferer, 0)
		}
	}
	if len(row) == 1 {
		ui.Body.AddRows(ui.NewRow(ui.NewCol(4, 0, row[0])))
	} else if len(row) == 2 {
		ui.Body.AddRows(ui.NewRow(
			ui.NewCol(4, 0, row[0]),
			ui.NewCol(4, 0, row[1]),
		))
	}

	ui.Body.Align()

	ui.Render(ui.Body)

	tm.PollEvent()
}

func fetchProcesses() []byte {
	ps := exec.Command("ps", "aux")

	grepTomcat := exec.Command("grep", "tomcat")
	grepTomcat.Stdin, _ = ps.StdoutPipe()

	grepCatalina := exec.Command("grep", "catalina")
	grepCatalina.Stdin, _ = grepTomcat.StdoutPipe()
	grepCatalinaOut, _ := grepCatalina.StdoutPipe()

	grepCatalina.Start()
	grepTomcat.Start()
	ps.Run()
	grepTomcat.Wait()

	out, err := ioutil.ReadAll(grepCatalinaOut)
	if err != nil {
		log.Fatalf("[x] Error when reading the output of the command: %s", err.Error())
	}

	grepCatalina.Wait()

	return out
}

func fetchCatalinaBases(processes []byte) []string {
	re := regexp.MustCompile("-Dcatalina\\.base=([a-zA-Z0-9/\\._-]+)")
	homes := make([]string, 0)

	r := bufio.NewReader(bytes.NewReader(processes))
	line, isPrefix, err := r.ReadLine()
	for err == nil && !isPrefix {
		s := string(line)
		homes = append(homes, re.FindStringSubmatch(s)[1])
		line, isPrefix, err = r.ReadLine()
	}

	return homes
}

func fetchWebapps(home string) []string {
	webapps := make([]string, 0)
	lsOut, err := exec.Command("ls", home+"/webapps").CombinedOutput()

	if err != nil {
		log.Fatalf("[x] Could not read the output of the command: %s", err.Error())
	}

	r := bufio.NewReader(bytes.NewReader(lsOut))
	line, isPrefix, err := r.ReadLine()
	for err == nil && !isPrefix {
		webapp := string(line)
		if isWebapp(webapp) {
			webapps = append(webapps, webapp)
		}

		line, isPrefix, err = r.ReadLine()
	}

	return webapps
}

func fetchTomcatPort(home string) (string, string) {
	var serverPort, httpPort, httpsPort string

	serverXml, _ := os.Open(home + "/conf/server.xml")
	defer serverXml.Close()

	reServer := regexp.MustCompile("<Server port=\"([0-9]+)\"")
	reHttp := regexp.MustCompile("<Connector port=\"([0-9]+)\" protocol=\"HTTP/1.1")
	reHttps := regexp.MustCompile("redirectPort=\"([0-9]+)\"")

	scanner := bufio.NewScanner(serverXml)
	for scanner.Scan() {
		line := scanner.Text()
		lineB := []byte(line)

		if reServer.Match(lineB) {
			serverPort = reServer.FindStringSubmatch(line)[1]
		}
		if reHttp.Match(lineB) {
			httpPort = reHttp.FindStringSubmatch(line)[1]
		}
		if reHttps.Match(lineB) {
			httpsPort = reHttps.FindStringSubmatch(line)[1]
		}
	}

	// Just in case
	if httpPort == "" {
		httpPort = serverPort[0:len(serverPort)-3] + "080"
	}
	if httpsPort == "" {
		httpsPort = serverPort[0:len(serverPort)-3] + "443"
	}

	return httpPort, httpsPort
}

func isWebapp(w string) bool {
	re := regexp.MustCompile(".+\\.war")
	return w != "docs" && w != "examples" && w != "host-manager" && w != "manager" && w != "ROOT" && !re.Match([]byte(w))
}
