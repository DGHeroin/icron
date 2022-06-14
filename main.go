package main

import (
    "errors"
    "flag"
    "fmt"
    "github.com/robfig/cron/v3"
    "io/ioutil"
    "log"
    "os"
    "os/exec"
    "os/user"
    "runtime"
    "strings"
    "time"
)

var (
    rule      = flag.String("r", "*/30 * 9-14 * * 1-5", "time spec rule")
    command   = flag.String("c", "echo hello", "command")
    f         = flag.String("f", "/etc/icron/icron.conf", "config file")
    isInstall = flag.Bool("i", false, "install")
    uninstall = flag.Bool("u", false, "uninstall")
)
var (
    c = cron.New(cron.WithSeconds())
)

func main() {
    flag.Parse()

    if *uninstall {
        checkLinuxUninstall()
        return
    }
    if *isInstall {
        checkLinuxSystemd()
        return
    }
    if isFileExists(*f) {
        runAsFile(*f)
    } else {
        startJob(*rule, *command)
    }

    c.Start()
    select {}
}

func runAsFile(filename string) {
    data, err := ioutil.ReadFile(filename)
    if err != nil {
        log.Fatalln(err)
    }
    content := string(data)
    lines := strings.Split(content, "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }
        tokens := strings.Split(line, " ")
        if len(tokens) < 7 {
            log.Println("conf parse error:", line)
            continue
        }
        rule := strings.Join(tokens[:6], " ")
        args := strings.Join(tokens[6:], " ")
        startJob(rule, args)
    }
}
func startJob(rule string, command string) {

    cmds := strings.Split(command, " ")
    var (
        name string
        args []string
    )
    sz := len(cmds)
    if sz == 0 {
        return
    }
    name = cmds[0]
    if sz > 1 {
        args = cmds[1:]
    }
    id, err := c.AddFunc(rule, func() {
        cmd := exec.Command(name, args...)
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr
        err := cmd.Run()
        if err != nil {
            fmt.Println(err)
            return
        }
    })
    if err != nil {
        fmt.Println(err)
        return
    }

    time.AfterFunc(time.Second, func() {
        entry := c.Entry(id)
        log.Printf("rule:[%v] args:[%v] next:%v", rule, command, entry.Next)
    })
}

func checkLinuxUninstall() {
    if runtime.GOOS != "linux" {
        return
    }
    filename := "/etc/systemd/system/icron.service"
    if !isFileExists(filename) {
        return
    }
    os.Remove(filename)
}
func checkLinuxSystemd() {
    if runtime.GOOS != "linux" {
        return
    }
    if !isRoot() {
        log.Println("[install] need root permission")
        return
    }
    filename := "/etc/systemd/system/icron.service"
    if !isFileExists(filename) {
        if err := ioutil.WriteFile(filename, []byte(systemdCfg), os.ModePerm); err != nil {
            log.Fatalln(err)
            return
        }
    }
}
func isRoot() bool {
    currentUser, err := user.Current()
    if err != nil {
        log.Fatalf(" Unable to get current user: %s", err)
    }
    return currentUser.Username == "root"
}

func isFileExists(filename string) bool {
    _, err := os.Stat(filename)
    return !errors.Is(err, os.ErrNotExist)
}

var (
    systemdCfg = `
[Unit]
Description=icron
After=network.target

[Service]
User=root
Type=simple
ExecStart=icron
WorkingDirectory=/etc/icron/
Restart=always
RestartSec=1s
LimitNOFILE=400000

[Install]
WantedBy=multi-user.target
`
)
