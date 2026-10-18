package garden

import (
    "database/sql"
    "net/http"
    "strconv"
    "strings"
    "time"
)
type Species struct {
    ID int64 `json:"id"`
    Name string `json:"name"`
}
type Plant struct {
    ID int64 `json:"id"`
    Name string `json:"name"`
    SpeciesID int64 `json:"species_id"`
    SpeciesName string `json:"species_name"`
    PlantedOn string `json:"planted_on"`
    Location string `json:"location"`
    Notes string `json:"notes"`
    Status string `json:"status"`
    PhotoCount int64 `json:"photo_count"`
    LatestPhotoID int64 `json:"latest_photo_id"`
    CreatedAt string `json:"created_at"`
}
const plantSelect = "SELECT p.id,p.name,COALESCE(p.species_id,0),COALESCE(s.name,''),p.planted_on,p.location,p.notes,p.status,p.created_at,"+
    "(SELECT count(*) FROM photos ph JOIN entries e ON e.id=ph.entry_id WHERE e.plant_id=p.id),"+
    "COALESCE((SELECT ph.id FROM photos ph JOIN entries e ON e.id=ph.entry_id WHERE e.plant_id=p.id ORDER BY e.observed_unix DESC,ph.id DESC LIMIT 1),0) "+
    "FROM plants p LEFT JOIN species s ON s.id=p.species_id "
func scanPlant(row interface{Scan(...any) error})(Plant,error){
    var p Plant
    err:=row.Scan(&p.ID,&p.Name,&p.SpeciesID,&p.SpeciesName,&p.PlantedOn,&p.Location,&p.Notes,&p.Status,&p.CreatedAt,&p.PhotoCount,&p.LatestPhotoID)
    return p,err
}
func(a *App) species(w http.ResponseWriter,r *http.Request,_ Session){
    switch r.Method{
    case "GET":
        rows,err:=a.db.Query("SELECT id,name FROM species ORDER BY name COLLATE NOCASE")
        if err!=nil{fail(w,500,"加载品种失败");return}
        defer rows.Close()
        result:=[]Species{}
        for rows.Next(){var s Species;if err:=rows.Scan(&s.ID,&s.Name);err!=nil{fail(w,500,"加载品种失败");return};result=append(result,s)}
        jsonResponse(w,200,result)
    case "POST":
        var in struct{Name string `json:"name"`}
        if err:=decode(r,&in);err!=nil{fail(w,400,err.Error());return}
        in.Name=strings.TrimSpace(in.Name)
        if in.Name==""||len([]rune(in.Name))>80{fail(w,400,"品种名称长度须为 1–80 个字符");return}
        res,err:=a.db.Exec("INSERT INTO species(name,created_at) VALUES(?,?)",in.Name,now())
        if err!=nil{fail(w,409,"品种已存在");return}
        id,_:=res.LastInsertId()
        jsonResponse(w,201,Species{ID:id,Name:in.Name})
    default:method(w,r,"GET","POST")
    }
}
func(a *App) speciesItem(w http.ResponseWriter,r *http.Request,_ Session){
    id,ok:=idFrom(r.URL.Path,"/api/v1/species/")
    if !ok{fail(w,404,"品种不存在");return}
    switch r.Method{
    case "PATCH":
        var in struct{Name string `json:"name"`}
        if err:=decode(r,&in);err!=nil{fail(w,400,err.Error());return}
        in.Name=strings.TrimSpace(in.Name)
        if in.Name==""||len([]rune(in.Name))>80{fail(w,400,"品种名称长度须为 1–80 个字符");return}
        res,err:=a.db.Exec("UPDATE species SET name=? WHERE id=?",in.Name,id)
        if err!=nil{fail(w,409,"品种重名");return}
        n,_:=res.RowsAffected();if n==0{fail(w,404,"品种不存在");return}
        jsonResponse(w,200,Species{ID:id,Name:in.Name})
    case "DELETE":
        // Unassigned plants remain; their species_id becomes NULL.
        res,err:=a.db.Exec("DELETE FROM species WHERE id=?",id)
        if err!=nil{fail(w,500,"删除品种失败");return}
        n,_:=res.RowsAffected();if n==0{fail(w,404,"品种不存在");return}
        jsonResponse(w,200,map[string]bool{"ok":true})
    default:method(w,r,"PATCH","DELETE")
    }
}
type plantInput struct{
    Name string `json:"name"`
    SpeciesID int64 `json:"species_id"`
    PlantedOn string `json:"planted_on"`
    Location string `json:"location"`
    Notes string `json:"notes"`
    Status string `json:"status"`
}
func checkPlant(p plantInput) string{
    if strings.TrimSpace(p.Name)==""||len([]rune(p.Name))>100{return "花卉名称须为 1–100 个字符"}
    if len([]rune(p.Location))>200||len([]rune(p.Notes))>3000{return "位置或备注过长"}
    if p.PlantedOn!=""{if _,err:=time.Parse("2006-01-02",p.PlantedOn);err!=nil{return "种植日期格式应为 YYYY-MM-DD"}}
    if p.Status!=""&&p.Status!="active"&&p.Status!="archived"{return "花卉状态无效"}
    if p.SpeciesID<0{return "品种无效"}
    return ""
}
func(a *App) plants(w http.ResponseWriter,r *http.Request,_ Session){
    switch r.Method{
    case "GET":
        q:=strings.TrimSpace(r.URL.Query().Get("q"))
        state:=r.URL.Query().Get("status")
        if state==""{state="active"}
        if state!="active"&&state!="archived"&&state!="all"{fail(w,400,"状态过滤条件无效");return}
        speciesID:=int64(0)
        if raw:=r.URL.Query().Get("species_id");raw!=""{
            n,err:=strconv.ParseInt(raw,10,64)
            if err!=nil||n<0{fail(w,400,"品种过滤条件无效");return}
            speciesID=n
        }
        sqlText:=plantSelect+" WHERE (?='all' OR p.status=?) AND (?=0 OR p.species_id=?) AND (?='' OR p.name LIKE '%'||?||'%' OR s.name LIKE '%'||?||'%' OR p.location LIKE '%'||?||'%') ORDER BY p.created_at DESC,p.id DESC"
        rows,err:=a.db.Query(sqlText,state,state,speciesID,speciesID,q,q,q,q)
        if err!=nil{fail(w,500,"加载花卉失败");return}
        defer rows.Close()
        result:=[]Plant{}
        for rows.Next(){p,err:=scanPlant(rows);if err!=nil{fail(w,500,"加载花卉失败");return};result=append(result,p)}
        jsonResponse(w,200,result)
    case "POST":
        var in plantInput
        if err:=decode(r,&in);err!=nil{fail(w,400,err.Error());return}
        if msg:=checkPlant(in);msg!=""{fail(w,400,msg);return}
        if in.Status==""{in.Status="active"}
        var species any
        if in.SpeciesID>0{species=in.SpeciesID}
        res,err:=a.db.Exec("INSERT INTO plants(name,species_id,planted_on,location,notes,status,created_at) VALUES(?,?,?,?,?,?,?)",strings.TrimSpace(in.Name),species,in.PlantedOn,strings.TrimSpace(in.Location),strings.TrimSpace(in.Notes),in.Status,now())
        if err!=nil{fail(w,400,"无法保存花卉，请检查品种");return}
        id,_:=res.LastInsertId()
        p,err:=scanPlant(a.db.QueryRow(plantSelect+"WHERE p.id=?",id))
        if err!=nil{fail(w,500,"读取花卉失败");return}
        jsonResponse(w,201,p)
    default:method(w,r,"GET","POST")
    }
}
func(a *App) plantItem(w http.ResponseWriter,r *http.Request,_ Session){
    id,ok:=idFrom(r.URL.Path,"/api/v1/plants/")
    if !ok{fail(w,404,"花卉不存在");return}
    switch r.Method{
    case "GET":
        p,err:=scanPlant(a.db.QueryRow(plantSelect+"WHERE p.id=?",id))
        if err!=nil{fail(w,404,"花卉不存在");return}
        jsonResponse(w,200,p)
    case "PATCH":
        p,err:=scanPlant(a.db.QueryRow(plantSelect+"WHERE p.id=?",id))
        if err==sql.ErrNoRows{fail(w,404,"花卉不存在");return}
        if err!=nil{fail(w,500,"读取花卉失败");return}
        var in plantInput
        if err:=decode(r,&in);err!=nil{fail(w,400,err.Error());return}
        // The editor sends all editable fields as one consistent document.
        if msg:=checkPlant(in);msg!=""{fail(w,400,msg);return}
        if in.Status==""{in.Status=p.Status}
        var species any
        if in.SpeciesID>0{species=in.SpeciesID}
        if _,err:=a.db.Exec("UPDATE plants SET name=?,species_id=?,planted_on=?,location=?,notes=?,status=? WHERE id=?",strings.TrimSpace(in.Name),species,in.PlantedOn,strings.TrimSpace(in.Location),strings.TrimSpace(in.Notes),in.Status,id);err!=nil{fail(w,400,"更新失败，请检查品种");return}
        updated,err:=scanPlant(a.db.QueryRow(plantSelect+"WHERE p.id=?",id))
        if err!=nil{fail(w,500,"读取花卉失败");return}
        jsonResponse(w,200,updated)
    case "DELETE":
        files,err:=a.filesForPlant(id)
        if err!=nil{fail(w,500,"读取照片失败");return}
        res,err:=a.db.Exec("DELETE FROM plants WHERE id=?",id)
        if err!=nil{fail(w,500,"删除花卉失败");return}
        n,_:=res.RowsAffected();if n==0{fail(w,404,"花卉不存在");return}
        a.removeFiles(files)
        jsonResponse(w,200,map[string]bool{"ok":true})
    default:method(w,r,"GET","PATCH","DELETE")
    }
}
