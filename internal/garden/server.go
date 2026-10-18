package garden

import (
    "crypto/rand"
    "crypto/sha256"
    "database/sql"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "log"
    "net"
    "net/http"
    "net/url"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"

    _ "modernc.org/sqlite"
)

type Config struct {
    DataDir string
    SetupToken string
    CookieSecure bool
}

type App struct {
    db *sql.DB
    cfg Config
    mux *http.ServeMux
    limiter *loginLimiter
}

type User struct {
    ID int64 `json:"id"`
    Username string `json:"username"`
    DisplayName string `json:"display_name"`
    Role string `json:"role"`
}

type Session struct {
    User User
    CSRF string
}

func New(cfg Config) (*App,error) {
    if cfg.DataDir=="" {return nil,errors.New("GD_DATA_DIR is required")}
    for _, sub:=range []string{"","originals","thumbs","previews","tmp"} {
        p:=filepath.Join(cfg.DataDir,sub)
        if err:=os.MkdirAll(p,0700);err!=nil{return nil,err}
    }
    path:=filepath.Join(cfg.DataDir,"garden.sqlite")
    db,err:=sql.Open("sqlite","file:"+filepath.ToSlash(path)+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
    if err!=nil{return nil,err}
    db.SetMaxOpenConns(1)
    a:=&App{db:db,cfg:cfg,mux:http.NewServeMux(),limiter:newLoginLimiter()}
    if err=a.migrate();err!=nil{db.Close();return nil,err}
    a.routes()
    return a,nil
}
func (a *App) Close() error {return a.db.Close()}
func (a *App) ServeHTTP(w http.ResponseWriter,r *http.Request) {
    w.Header().Set("X-Content-Type-Options","nosniff")
    w.Header().Set("Referrer-Policy","same-origin")
    w.Header().Set("X-Frame-Options","DENY")
    w.Header().Set("Content-Security-Policy","frame-ancestors 'none'")
    if r.URL.Path=="/api/v1/health" {if err:=a.db.PingContext(r.Context());err!=nil{fail(w,503,"数据库不可用");return};jsonResponse(w,200,map[string]any{"ok":true});return}
    if strings.HasPrefix(r.URL.Path,"/api/v1/") && r.Method!="GET" && r.Method!="HEAD" {
        origin:=r.Header.Get("Origin")
        if origin!="" {
            parsed,err:=url.Parse(origin)
            if err!=nil || !strings.EqualFold(parsed.Host,r.Host) || (parsed.Scheme!="http"&&parsed.Scheme!="https") {
                fail(w,403,"请求来源不被允许");return
            }
        }
    }
    a.mux.ServeHTTP(w,r)
}
func (a *App) migrate() error {
    statements:=[]string{
        "CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE COLLATE NOCASE, display_name TEXT NOT NULL, password_hash TEXT NOT NULL, role TEXT NOT NULL CHECK(role IN ('admin','member')), disabled INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL)",
        "CREATE TABLE IF NOT EXISTS sessions (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, csrf TEXT NOT NULL, expires_unix INTEGER NOT NULL)",
        "CREATE INDEX IF NOT EXISTS idx_sessions_exp ON sessions(expires_unix)",
        "CREATE TABLE IF NOT EXISTS species (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE COLLATE NOCASE, created_at TEXT NOT NULL)",
        "CREATE TABLE IF NOT EXISTS plants (id INTEGER PRIMARY KEY, name TEXT NOT NULL, species_id INTEGER REFERENCES species(id) ON DELETE SET NULL, planted_on TEXT NOT NULL DEFAULT '', location TEXT NOT NULL DEFAULT '', notes TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','archived')), created_at TEXT NOT NULL)",
        "CREATE INDEX IF NOT EXISTS idx_plants_species ON plants(species_id,status)",
        "CREATE TABLE IF NOT EXISTS entries (id INTEGER PRIMARY KEY, plant_id INTEGER NOT NULL REFERENCES plants(id) ON DELETE CASCADE, author_id INTEGER REFERENCES users(id) ON DELETE SET NULL, observed_at TEXT NOT NULL, observed_unix INTEGER NOT NULL, note TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL)",
        "CREATE INDEX IF NOT EXISTS idx_entries_time ON entries(plant_id,observed_unix DESC,id DESC)",
        "CREATE TABLE IF NOT EXISTS photos (id INTEGER PRIMARY KEY, entry_id INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE, storage_name TEXT NOT NULL UNIQUE, thumb_name TEXT NOT NULL DEFAULT '', original_name TEXT NOT NULL, mime TEXT NOT NULL, size_bytes INTEGER NOT NULL, created_at TEXT NOT NULL)",
        "CREATE INDEX IF NOT EXISTS idx_photos_entry ON photos(entry_id)",
    }
    for _,s:=range statements {
        if _,err:=a.db.Exec(s);err!=nil{return fmt.Errorf("migrate: %w: %.80s",err,s)}
    }
    return nil
}
func (a *App) routes() {
    a.mux.HandleFunc("/api/v1/bootstrap",a.bootstrap)
    a.mux.HandleFunc("/api/v1/setup",a.setup)
    a.mux.HandleFunc("/api/v1/login",a.login)
    a.mux.HandleFunc("/api/v1/logout",a.auth(a.logout))
    a.mux.HandleFunc("/api/v1/me",a.auth(a.me))
    a.mux.HandleFunc("/api/v1/password",a.auth(a.changePassword))
    a.mux.HandleFunc("/api/v1/users",a.auth(a.users))
    a.mux.HandleFunc("/api/v1/users/",a.auth(a.userItem))
    a.mux.HandleFunc("/api/v1/species",a.auth(a.species))
    a.mux.HandleFunc("/api/v1/species/",a.auth(a.speciesItem))
    a.mux.HandleFunc("/api/v1/plants",a.auth(a.plants))
    a.mux.HandleFunc("/api/v1/plants/",a.auth(a.plantItem))
    a.mux.HandleFunc("/api/v1/entries",a.auth(a.entries))
    a.mux.HandleFunc("/api/v1/entries/",a.auth(a.entryItem))
    a.mux.HandleFunc("/api/v1/photos/",a.auth(a.photoFile))
    a.mux.HandleFunc("/api/v1/calendar",a.auth(a.calendar))
    a.mux.HandleFunc("/api/v1/calendar/day",a.auth(a.calendarDay))
    a.mux.HandleFunc("/api/v1/export",a.auth(a.exportZip))
}
type handler func(http.ResponseWriter,*http.Request,Session)
func (a *App) auth(next handler) http.HandlerFunc {
    return func(w http.ResponseWriter,r *http.Request) {
        cookie,err:=r.Cookie("gd_session")
        if err!=nil || len(cookie.Value)>200 {fail(w,401,"请先登录");return}
        hash:=sha256.Sum256([]byte(cookie.Value))
        sess:=Session{}
        err=a.db.QueryRow("SELECT u.id,u.username,u.display_name,u.role,s.csrf FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.expires_unix>? AND u.disabled=0",hex.EncodeToString(hash[:]),time.Now().Unix()).Scan(&sess.User.ID,&sess.User.Username,&sess.User.DisplayName,&sess.User.Role,&sess.CSRF)
        if err!=nil {fail(w,401,"会话已失效，请重新登录");return}
        if r.Method!="GET" && r.Method!="HEAD" {
            supplied:=r.Header.Get("X-CSRF-Token")
            if supplied=="" || !constantEqual(supplied,sess.CSRF) {fail(w,403,"CSRF 校验失败");return}
        }
        next(w,r,sess)
    }
}
func constantEqual(a,b string) bool {
    aa:=sha256.Sum256([]byte(a));bb:=sha256.Sum256([]byte(b))
    var n byte
    for i:=0;i<len(aa);i++ {n|=aa[i]^bb[i]}
    return n==0
}
func randomToken() (string,error) {
    b:=make([]byte,32)
    if _,err:=rand.Read(b);err!=nil{return "",err}
    return hex.EncodeToString(b),nil
}
func jsonResponse(w http.ResponseWriter,status int,value any) {
    w.Header().Set("Content-Type","application/json; charset=utf-8")
    w.Header().Set("Cache-Control","no-store")
    w.WriteHeader(status)
    if err:=json.NewEncoder(w).Encode(value);err!=nil{log.Printf("write JSON: %v",err)}
}
func fail(w http.ResponseWriter,status int,msg string) {jsonResponse(w,status,map[string]string{"error":msg})}
func decode(r *http.Request,dst any) error {
    if !strings.HasPrefix(r.Header.Get("Content-Type"),"application/json") {return errors.New("需要 application/json")}
    body:=io.LimitReader(r.Body,(1<<20)+1)
    decoder:=json.NewDecoder(body)
    decoder.DisallowUnknownFields()
    if err:=decoder.Decode(dst);err!=nil{return err}
    var extra any
    if err:=decoder.Decode(&extra);err!=io.EOF {return errors.New("JSON 数据过长或包含额外内容")}
    return nil
}
func method(w http.ResponseWriter,r *http.Request,allowed ...string) bool {
    for _,m:=range allowed {if r.Method==m{return true}}
    w.Header().Set("Allow",strings.Join(allowed,", "))
    fail(w,405,"不支持的请求方法")
    return false
}
func idFrom(path,prefix string) (int64,bool) {
    raw:=strings.TrimPrefix(path,prefix)
    if raw==""||strings.Contains(raw,"/") {return 0,false}
    id,err:=strconv.ParseInt(raw,10,64)
    return id,id>0&&err==nil
}
func now() string {return time.Now().UTC().Format(time.RFC3339)}
func clientIP(r *http.Request) string {
    host,_,err:=net.SplitHostPort(r.RemoteAddr)
    if err!=nil {return r.RemoteAddr}
    return host
}
