package main

import (
    "github.com/spf13/cobra"
    "os/exec"
    "io/ioutil"
    "log"
    "bufio"
    "bytes"
    "regexp"
    "os"
    "github.com/gizak/termui"
    "github.com/nsf/termbox-go"
)

func main() {
    var rootCmd = cobra.Command{
        Use: "",
        Long: "",
        Run: whichTomcat,
    }
    rootCmd.Execute()
}

func whichTomcat(cmd *cobra.Command, args []string) {
    processes := fetchProcesses()
    homes := fetchCatalinaHomes(processes)

    tomcats := make([]*Tomcat, 0)
    for _, home := range homes {
        webapps := fetchWebapps(home)
        httpPort, httpsPort := fetchTomcatPort(home)
        tomcats = append(tomcats, &Tomcat{
            Home: home,
            Webapps: webapps,
            HttpPort: httpPort,
            HttpsPort: httpsPort,
        })
    }
    displayTomcats(tomcats)
}

type Tomcat struct {
    Home string
    Webapps []string
    HttpPort string
    HttpsPort string
}

func displayTomcats(tomcats []*Tomcat) {
    err := termui.Init()
    if err != nil {
        panic(err)
    }
    defer termui.Close()

    termui.UseTheme("helloworld")

    renders := make([]termui.Bufferer, 0)

    for index, tomcat := range tomcats {
        list := make([]string, len(tomcat.Webapps) + 3)
        list[0] = "[HTTP]  " + tomcat.HttpPort
        list[1] = "[HTTPS] " + tomcat.HttpsPort
        list[2] = "------------------------------------------------"
        copy(list[3:], tomcat.Webapps)

        ls := termui.NewList()
        ls.Items = list
        ls.ItemFgColor = termui.ColorYellow
        ls.Border.Label = tomcat.Home
        ls.Height = 15
        ls.Width = 50
        ls.Y = index * 10

        renders = append(renders, ls)
    }

    termui.Render(renders...)

    termbox.PollEvent()
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

func fetchCatalinaHomes(processes []byte) []string {
    re := regexp.MustCompile("-Dcatalina\\.home=([a-zA-Z0-9/\\._-]+)")
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
    lsOut, err := exec.Command("ls", home + "/webapps").CombinedOutput()

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
    var httpPort, httpsPort string

    serverXml, _ := os.Open(home + "/conf/server.xml")
    defer serverXml.Close()

    reHttp := regexp.MustCompile("<Connector port=\"([0-9]+)\" protocol=\"HTTP/1.1")
    reHttps := regexp.MustCompile("redirectPort=\"([0-9]+)\"")

    scanner := bufio.NewScanner(serverXml)
    for scanner.Scan() {
        line := scanner.Text()
        lineB := []byte(line)

        if reHttp.Match(lineB) {
            httpPort = reHttp.FindStringSubmatch(line)[1]
        }
        if reHttps.Match(lineB) {
            httpsPort = reHttps.FindStringSubmatch(line)[1]
        }
    }

    return httpPort, httpsPort
}

func isWebapp(w string) bool {
    re := regexp.MustCompile(".+\\.war")
    return w != "docs" && w != "examples" && w != "host-manager" && w != "manager" && w != "ROOT" && !re.Match([]byte(w))
}
