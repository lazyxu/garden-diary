package main

import (
    "log"
    "net/http"
    "io"
    "os"
    "time"

    "github.com/lazyxu/garden-diary/internal/garden"
)

func main() {
    if len(os.Args)>1 && os.Args[1]=="healthcheck" {
        client:=&http.Client{Timeout:3*time.Second}
        response,err:=client.Get("http://127.0.0.1:8080/api/v1/health")
        if err!=nil{log.Fatal(err)}
        defer response.Body.Close()
        io.Copy(io.Discard,response.Body)
        if response.StatusCode!=http.StatusOK{os.Exit(1)}
        return
    }
    app, err := garden.New(garden.Config{
        DataDir: env("GD_DATA_DIR", "./data"),
        SetupToken: os.Getenv("GD_SETUP_TOKEN"),
        CookieSecure: os.Getenv("GD_COOKIE_SECURE") == "true",
    })
    if err != nil { log.Fatal(err) }
    defer app.Close()
    listen := env("GD_LISTEN", ":8080")
    server := &http.Server{Addr:listen, Handler:app, ReadHeaderTimeout:10*time.Second, IdleTimeout:90*time.Second}
    log.Printf("garden-diary listening on %s", listen)
    log.Fatal(server.ListenAndServe())
}

func env(key, fallback string) string {
    if v:=os.Getenv(key);v!="" { return v }
    return fallback
}
