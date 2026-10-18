package garden

import (
    "crypto/sha256"
    "database/sql"
    "encoding/hex"
    "errors"
    "net/http"
    "regexp"
    "strings"
    "sync"
    "time"

    "golang.org/x/crypto/bcrypt"
)
type loginLimiter struct {
    mu sync.Mutex
    seen map[string]attempt
}
type attempt struct{ count int; reset time.Time }
func newLoginLimiter()*loginLimiter{return &loginLimiter{seen:make(map[string]attempt)}}
func (l *loginLimiter) blocked(key string) bool {
    l.mu.Lock();defer l.mu.Unlock()
    a:=l.seen[key]
    return time.Now().Before(a.reset)&&a.count>=8
}
func(l *loginLimiter) failure(key string){
    l.mu.Lock();defer l.mu.Unlock()
    if len(l.seen)>10000 {l.seen=make(map[string]attempt)}
    a:=l.seen[key];if time.Now().After(a.reset){a=attempt{reset:time.Now().Add(15*time.Minute)}}
    a.count++;l.seen[key]=a
}
func(l *loginLimiter) clear(key string){l.mu.Lock();delete(l.seen,key);l.mu.Unlock()}
var userPattern=regexp.MustCompile("^[a-zA-Z0-9_-]{3,32}$")
func validateIdentity(username,display,password string) error{
    if !userPattern.MatchString(username) {return errors.New("用户名须为 3–32 位英文字母、数字、_ 或 -")}
    if len([]rune(strings.TrimSpace(display)))<1||len([]rune(display))>60{return errors.New("显示名称须为 1–60 个字符")}
    if len(password)<10||len(password)>72{return errors.New("密码长度须为 10–72 个字节")}
    return nil
}
type credentials struct{
    Username string `json:"username"`
    Password string `json:"password"`
    DisplayName string `json:"display_name"`
    SetupToken string `json:"setup_token"`
    Role string `json:"role"`
}
func(a *App) bootstrap(w http.ResponseWriter,r *http.Request){
    if !method(w,r,"GET"){return}
    var count int
    if err:=a.db.QueryRow("SELECT count(*) FROM users").Scan(&count);err!=nil{fail(w,500,"查询初始化状态失败");return}
    jsonResponse(w,200,map[string]any{"needs_setup":count==0})
}
func(a *App) setup(w http.ResponseWriter,r *http.Request){
    if !method(w,r,"POST"){return}
    var in credentials
    if err:=decode(r,&in);err!=nil{fail(w,400,err.Error());return}
    if a.cfg.SetupToken==""||!constantEqual(in.SetupToken,a.cfg.SetupToken){fail(w,403,"初始化口令错误");return}
    if err:=validateIdentity(in.Username,in.DisplayName,in.Password);err!=nil{fail(w,400,err.Error());return}
    hash,err:=bcrypt.GenerateFromPassword([]byte(in.Password),bcrypt.DefaultCost)
    if err!=nil{fail(w,500,"密码处理失败");return}
    tx,err:=a.db.Begin()
    if err!=nil{fail(w,500,"无法启动初始化事务");return}
    defer tx.Rollback()
    var count int
    if err=tx.QueryRow("SELECT count(*) FROM users").Scan(&count);err!=nil||count!=0{fail(w,409,"花园已经初始化");return}
    result,err:=tx.Exec("INSERT INTO users(username,display_name,password_hash,role,created_at) VALUES(?,?,?,?,?)",strings.TrimSpace(in.Username),strings.TrimSpace(in.DisplayName),string(hash),"admin",now())
    if err!=nil{fail(w,409,"无法创建管理员账号");return}
    id,_:=result.LastInsertId()
    if err=tx.Commit();err!=nil{fail(w,500,"保存账号失败");return}
    a.issueSession(w,r,User{ID:id,Username:in.Username,DisplayName:in.DisplayName,Role:"admin"})
}
func(a *App) login(w http.ResponseWriter,r *http.Request) {
    if !method(w,r,"POST"){return}
    var in credentials
    if err:=decode(r,&in);err!=nil{fail(w,400,err.Error());return}
    key:=clientIP(r)+":"+strings.ToLower(in.Username)
    if a.limiter.blocked(key){fail(w,429,"登录尝试过多，请稍后再试");return}
    var u User
    var hash string
    err:=a.db.QueryRow("SELECT id,username,display_name,role,password_hash FROM users WHERE username=? AND disabled=0",in.Username).Scan(&u.ID,&u.Username,&u.DisplayName,&u.Role,&hash)
    if err!=nil||bcrypt.CompareHashAndPassword([]byte(hash),[]byte(in.Password))!=nil {
        a.limiter.failure(key)
        fail(w,401,"账号或密码错误");return
    }
    a.limiter.clear(key)
    a.issueSession(w,r,u)
}
func(a *App) issueSession(w http.ResponseWriter,r *http.Request,u User){
    token,err:=randomToken();if err!=nil{fail(w,500,"创建会话失败");return}
    csrf,err:=randomToken();if err!=nil{fail(w,500,"创建会话失败");return}
    hash:=sha256.Sum256([]byte(token))
    expires:=time.Now().Add(30*24*time.Hour)
    if _,err=a.db.Exec("INSERT INTO sessions(token_hash,user_id,csrf,expires_unix) VALUES(?,?,?,?)",hex.EncodeToString(hash[:]),u.ID,csrf,expires.Unix());err!=nil{fail(w,500,"保存会话失败");return}
    http.SetCookie(w,&http.Cookie{Name:"gd_session",Value:token,Path:"/",HttpOnly:true,Secure:a.cfg.CookieSecure||r.Header.Get("X-Forwarded-Proto")=="https",SameSite:http.SameSiteLaxMode,Expires:expires,MaxAge:30*24*3600})
    jsonResponse(w,200,map[string]any{"user":u,"csrf":csrf})
}
func(a *App) logout(w http.ResponseWriter,r *http.Request,s Session){
    if !method(w,r,"POST"){return}
    cookie,_:=r.Cookie("gd_session")
    if cookie!=nil {
        hash:=sha256.Sum256([]byte(cookie.Value))
        a.db.Exec("DELETE FROM sessions WHERE token_hash=?",hex.EncodeToString(hash[:]))
    }
    http.SetCookie(w,&http.Cookie{Name:"gd_session",Path:"/",HttpOnly:true,Secure:a.cfg.CookieSecure,SameSite:http.SameSiteLaxMode,MaxAge:-1})
    jsonResponse(w,200,map[string]bool{"ok":true})
}
func(a *App) me(w http.ResponseWriter,r *http.Request,s Session){
    if !method(w,r,"GET"){return}
    jsonResponse(w,200,map[string]any{"user":s.User,"csrf":s.CSRF})
}
func(a *App) changePassword(w http.ResponseWriter,r *http.Request,s Session){
    if !method(w,r,"POST"){return}
    var in struct{Old string `json:"old_password"`;New string `json:"new_password"`}
    if err:=decode(r,&in);err!=nil{fail(w,400,err.Error());return}
    if len(in.New)<10||len(in.New)>72{fail(w,400,"新密码长度须为 10–72 个字节");return}
    var oldHash string
    if err:=a.db.QueryRow("SELECT password_hash FROM users WHERE id=?",s.User.ID).Scan(&oldHash);err!=nil||bcrypt.CompareHashAndPassword([]byte(oldHash),[]byte(in.Old))!=nil{fail(w,403,"原密码错误");return}
    hash,err:=bcrypt.GenerateFromPassword([]byte(in.New),bcrypt.DefaultCost);if err!=nil{fail(w,500,"密码处理失败");return}
    if _,err=a.db.Exec("UPDATE users SET password_hash=? WHERE id=?",string(hash),s.User.ID);err!=nil{fail(w,500,"更新密码失败");return}
    a.db.Exec("DELETE FROM sessions WHERE user_id=? AND token_hash<> (SELECT token_hash FROM sessions WHERE user_id=? AND csrf=? LIMIT 1)",s.User.ID,s.User.ID,s.CSRF)
    jsonResponse(w,200,map[string]bool{"ok":true})
}
func(a *App) users(w http.ResponseWriter,r *http.Request,s Session){
    switch r.Method {
    case "GET":
        rows,err:=a.db.Query("SELECT id,username,display_name,role,disabled FROM users ORDER BY id")
        if err!=nil{fail(w,500,"加载成员失败");return}
        defer rows.Close()
        users:=[]map[string]any{}
        for rows.Next(){
            var u User;var disabled int
            if err:=rows.Scan(&u.ID,&u.Username,&u.DisplayName,&u.Role,&disabled);err!=nil{fail(w,500,"加载成员失败");return}
            users=append(users,map[string]any{"id":u.ID,"username":u.Username,"display_name":u.DisplayName,"role":u.Role,"disabled":disabled!=0})
        }
        jsonResponse(w,200,users)
    case "POST":
        if s.User.Role!="admin"{fail(w,403,"只有管理员可以添加家庭成员");return}
        var in credentials
        if err:=decode(r,&in);err!=nil{fail(w,400,err.Error());return}
        if err:=validateIdentity(in.Username,in.DisplayName,in.Password);err!=nil{fail(w,400,err.Error());return}
        if in.Role==""{in.Role="member"}
        if in.Role!="admin"&&in.Role!="member"{fail(w,400,"角色无效");return}
        hash,err:=bcrypt.GenerateFromPassword([]byte(in.Password),bcrypt.DefaultCost);if err!=nil{fail(w,500,"密码处理失败");return}
        result,err:=a.db.Exec("INSERT INTO users(username,display_name,password_hash,role,created_at) VALUES(?,?,?,?,?)",in.Username,strings.TrimSpace(in.DisplayName),string(hash),in.Role,now())
        if err!=nil{fail(w,409,"用户名已存在");return}
        id,_:=result.LastInsertId()
        jsonResponse(w,201,map[string]any{"id":id})
    default:method(w,r,"GET","POST")
    }
}
func(a *App) userItem(w http.ResponseWriter,r *http.Request,s Session){
    if !method(w,r,"PATCH"){return}
    if s.User.Role!="admin"{fail(w,403,"只有管理员可以管理成员");return}
    id,ok:=idFrom(r.URL.Path,"/api/v1/users/")
    if !ok{fail(w,404,"成员不存在");return}
    if id==s.User.ID {fail(w,400,"不能禁用自己的账号");return}
    var in struct{Disabled *bool `json:"disabled"`}
    if err:=decode(r,&in);err!=nil||in.Disabled==nil{fail(w,400,"请提供 disabled");return}
    state:=0;if *in.Disabled{state=1}
    // Keep at least one active administrator.
    var role string
    if err:=a.db.QueryRow("SELECT role FROM users WHERE id=?",id).Scan(&role);err!=nil{if err==sql.ErrNoRows{fail(w,404,"成员不存在")}else{fail(w,500,"读取成员失败")};return}
    if role=="admin"&&state==1 {
        var count int
        a.db.QueryRow("SELECT count(*) FROM users WHERE role='admin' AND disabled=0").Scan(&count)
        if count<=1{fail(w,409,"必须保留至少一名管理员");return}
    }
    if _,err:=a.db.Exec("UPDATE users SET disabled=? WHERE id=?",state,id);err!=nil{fail(w,500,"更新成员失败");return}
    if state==1{a.db.Exec("DELETE FROM sessions WHERE user_id=?",id)}
    jsonResponse(w,200,map[string]bool{"ok":true})
}
