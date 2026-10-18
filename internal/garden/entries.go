package garden

import (
    "errors"
    "fmt"
    "net/http"
    "strconv"
    "strings"
    "time"
)
type Photo struct{
    ID int64 `json:"id"`
    Name string `json:"name"`
    Mime string `json:"mime"`
    Size int64 `json:"size"`
}
type Entry struct{
    ID int64 `json:"id"`
    PlantID int64 `json:"plant_id"`
    Author string `json:"author"`
    ObservedAt string `json:"observed_at"`
    Note string `json:"note"`
    Photos []Photo `json:"photos"`
}
func(a *App) entries(w http.ResponseWriter,r *http.Request,s Session){
    switch r.Method {
    case "GET":
        plantID,err:=strconv.ParseInt(r.URL.Query().Get("plant_id"),10,64)
        if err!=nil||plantID<1{fail(w,400,"需要花卉 ID");return}
        limit:=50
        if raw:=r.URL.Query().Get("limit");raw!="" {
            n,err:=strconv.Atoi(raw);if err!=nil||n<1||n>200{fail(w,400,"limit 应为 1–200");return};limit=n
        }
        offset:=0
        if raw:=r.URL.Query().Get("offset");raw!=""{
            n,err:=strconv.Atoi(raw);if err!=nil||n<0{fail(w,400,"offset 无效");return};offset=n
        }
        rows,err:=a.db.Query("SELECT e.id,e.plant_id,COALESCE(u.display_name,'已删除成员'),e.observed_at,e.note FROM entries e LEFT JOIN users u ON u.id=e.author_id WHERE e.plant_id=? ORDER BY e.observed_unix DESC,e.id DESC LIMIT ? OFFSET ?",plantID,limit+1,offset)
        if err!=nil{fail(w,500,"加载日记失败");return}
        entries:=[]Entry{}
        for rows.Next(){
            var item Entry;item.Photos=[]Photo{}
            if err=rows.Scan(&item.ID,&item.PlantID,&item.Author,&item.ObservedAt,&item.Note);err!=nil{break}
            entries=append(entries,item)
        }
        readErr:=rows.Err()
        rows.Close()
        if err!=nil||readErr!=nil{fail(w,500,"读取日记失败");return}
        hasMore:=len(entries)>limit
        if hasMore{entries=entries[:limit]}
        for i:=range entries{
            photos,err:=a.photosForEntry(entries[i].ID)
            if err!=nil{fail(w,500,"读取日记照片失败");return}
            entries[i].Photos=photos
        }
        jsonResponse(w,200,map[string]any{"items":entries,"has_more":hasMore})
    case "POST":
        a.createEntry(w,r,s)
    default:method(w,r,"GET","POST")
    }
}
func parseObserved(s string)(time.Time,error) {
    if s==""{return time.Time{},errors.New("必须提供记录时间")}
    t,err:=time.Parse(time.RFC3339,s)
    if err!=nil{return time.Time{},errors.New("记录时间必须为 ISO 8601，包含时区")}
    if t.After(time.Now().Add(24*time.Hour)){return time.Time{},errors.New("记录时间不能超过未来 24 小时")}
    return t,nil
}
func(a *App) createEntry(w http.ResponseWriter,r *http.Request,s Session){
    r.Body=http.MaxBytesReader(w,r.Body,128<<20)
    if err:=r.ParseMultipartForm(16<<20);err!=nil{fail(w,413,"上传过大，最多 8 张照片、总计 128 MiB");return}
    if r.MultipartForm!=nil{defer r.MultipartForm.RemoveAll()}
    plantID,err:=strconv.ParseInt(r.FormValue("plant_id"),10,64)
    if err!=nil||plantID<1{fail(w,400,"花卉 ID 无效");return}
    var count int
    if err:=a.db.QueryRow("SELECT count(*) FROM plants WHERE id=?",plantID).Scan(&count);err!=nil||count==0{fail(w,404,"花卉不存在");return}
    observed,err:=parseObserved(r.FormValue("observed_at"));if err!=nil{fail(w,400,err.Error());return}
    note:=strings.TrimSpace(r.FormValue("note"))
    if len([]rune(note))>5000{fail(w,400,"备注不得超过 5000 字");return}
    headers:=r.MultipartForm.File["photos"]
    if len(headers)<1||len(headers)>8{fail(w,400,"每篇日记需上传 1–8 张照片");return}
    tx,err:=a.db.Begin()
    if err!=nil{fail(w,500,"无法创建日记");return}
    defer tx.Rollback()
    res,err:=tx.Exec("INSERT INTO entries(plant_id,author_id,observed_at,observed_unix,note,created_at) VALUES(?,?,?,?,?,?)",plantID,s.User.ID,r.FormValue("observed_at"),observed.Unix(),note,now())
    if err!=nil{fail(w,500,"保存日记失败");return}
    entryID,_:=res.LastInsertId()
    created:=[]storedPhoto{}
    committed:=false
    defer func(){if !committed{for _,p:=range created{a.removeFiles([]fileNames{{Original:p.StorageName,Thumb:p.ThumbName}})}}}()
    for _,hdr:=range headers {
        p,err:=a.storeUpload(hdr)
        if err!=nil{fail(w,400,fmt.Sprintf("照片 %s: %v",hdr.Filename,err));return}
        created=append(created,p)
        _,err=tx.Exec("INSERT INTO photos(entry_id,storage_name,thumb_name,original_name,mime,size_bytes,created_at) VALUES(?,?,?,?,?,?,?)",entryID,p.StorageName,p.ThumbName,p.OriginalName,p.Mime,p.Size,now())
        if err!=nil{fail(w,500,"保存照片信息失败");return}
    }
    if err:=tx.Commit();err!=nil{fail(w,500,"提交日记失败");return}
    committed=true
    jsonResponse(w,201,map[string]any{"id":entryID})
}
func(a *App) photosForEntry(entryID int64)([]Photo,error){
    rows,err:=a.db.Query("SELECT id,original_name,mime,size_bytes FROM photos WHERE entry_id=? ORDER BY id",entryID)
    if err!=nil{return nil,err}
    defer rows.Close()
    result:=[]Photo{}
    for rows.Next(){var p Photo;if err:=rows.Scan(&p.ID,&p.Name,&p.Mime,&p.Size);err!=nil{return nil,err};result=append(result,p)}
    return result,rows.Err()
}
func(a *App) entryItem(w http.ResponseWriter,r *http.Request,_ Session){
    id,ok:=idFrom(r.URL.Path,"/api/v1/entries/")
    if !ok{fail(w,404,"日记不存在");return}
    switch r.Method {
    case "PATCH":
        var in struct{ObservedAt string `json:"observed_at"`;Note string `json:"note"`}
        if err:=decode(r,&in);err!=nil{fail(w,400,err.Error());return}
        t,err:=parseObserved(in.ObservedAt);if err!=nil{fail(w,400,err.Error());return}
        if len([]rune(in.Note))>5000{fail(w,400,"备注不得超过 5000 字");return}
        res,err:=a.db.Exec("UPDATE entries SET observed_at=?,observed_unix=?,note=? WHERE id=?",in.ObservedAt,t.Unix(),strings.TrimSpace(in.Note),id)
        if err!=nil{fail(w,500,"更新日记失败");return}
        n,_:=res.RowsAffected();if n==0{fail(w,404,"日记不存在");return}
        jsonResponse(w,200,map[string]bool{"ok":true})
    case "DELETE":
        files,err:=a.filesForEntry(id)
        if err!=nil{fail(w,500,"读取日记照片失败");return}
        res,err:=a.db.Exec("DELETE FROM entries WHERE id=?",id)
        if err!=nil{fail(w,500,"删除日记失败");return}
        n,_:=res.RowsAffected();if n==0{fail(w,404,"日记不存在");return}
        a.removeFiles(files)
        jsonResponse(w,200,map[string]bool{"ok":true})
    default:method(w,r,"PATCH","DELETE")
    }
}
func(a *App) calendar(w http.ResponseWriter,r *http.Request,_ Session) {
    if !method(w,r,"GET"){return}
    var plantID int64
    raw:=r.URL.Query().Get("plant_id")
    if raw!=""{id,err:=strconv.ParseInt(raw,10,64);if err!=nil||id<1{fail(w,400,"花卉 ID 无效");return};plantID=id}
    rows,err:=a.db.Query("SELECT substr(observed_at,1,10) day,count(*) FROM entries WHERE (?=0 OR plant_id=?) GROUP BY day ORDER BY day DESC",plantID,plantID)
    if err!=nil{fail(w,500,"加载日历失败");return}
    defer rows.Close()
    result:=[]map[string]any{}
    for rows.Next(){var day string;var count int;if err:=rows.Scan(&day,&count);err!=nil{fail(w,500,"加载日历失败");return};result=append(result,map[string]any{"day":day,"count":count})}
    jsonResponse(w,200,result)
}
