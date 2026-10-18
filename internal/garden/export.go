package garden

import (
    "archive/zip"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "time"
)
func(a *App) exportZip(w http.ResponseWriter,r *http.Request,s Session){
    if !method(w,r,"GET"){return}
    if s.User.Role!="admin"{fail(w,403,"只有管理员可以导出整个家庭花园");return}
    manifest:=map[string]any{
        "format":"garden-diary-export-v1",
        "exported_at":now(),
    }
    species:=[]map[string]any{}
    rows,err:=a.db.Query("SELECT id,name FROM species ORDER BY id")
    if err!=nil{fail(w,500,"导出品种失败");return}
    for rows.Next(){var id int64;var name string;if err:=rows.Scan(&id,&name);err!=nil{rows.Close();fail(w,500,"导出品种失败");return};species=append(species,map[string]any{"id":id,"name":name})}
    rows.Close()
    manifest["species"]=species
    plants:=[]Plant{}
    rows,err=a.db.Query(plantSelect+"ORDER BY p.id")
    if err!=nil{fail(w,500,"导出花卉失败");return}
    for rows.Next(){p,err:=scanPlant(rows);if err!=nil{rows.Close();fail(w,500,"导出花卉失败");return};plants=append(plants,p)}
    rows.Close()
    manifest["plants"]=plants
    entries:=[]map[string]any{}
    rows,err=a.db.Query("SELECT e.id,e.plant_id,COALESCE(u.display_name,'已删除成员'),e.observed_at,e.note FROM entries e LEFT JOIN users u ON u.id=e.author_id ORDER BY e.id")
    if err!=nil{fail(w,500,"导出日记失败");return}
    for rows.Next(){
        var id,plantID int64;var author,observed,note string
        if err:=rows.Scan(&id,&plantID,&author,&observed,&note);err!=nil{rows.Close();fail(w,500,"导出日记失败");return}
        entries=append(entries,map[string]any{"id":id,"plant_id":plantID,"author":author,"observed_at":observed,"note":note})
    }
    rows.Close()
    manifest["entries"]=entries
    type exportPhoto struct{ID int64;EntryID int64;Storage,OriginalName,Mime string}
    photos:=[]exportPhoto{}
    rows,err=a.db.Query("SELECT id,entry_id,storage_name,original_name,mime FROM photos ORDER BY id")
    if err!=nil{fail(w,500,"导出照片失败");return}
    for rows.Next(){var p exportPhoto;if err:=rows.Scan(&p.ID,&p.EntryID,&p.Storage,&p.OriginalName,&p.Mime);err!=nil{rows.Close();fail(w,500,"导出照片失败");return};photos=append(photos,p)}
    rows.Close()
    photoMeta:=[]map[string]any{}
    for _,p:=range photos{photoMeta=append(photoMeta,map[string]any{"id":p.ID,"entry_id":p.EntryID,"original_name":p.OriginalName,"mime":p.Mime,"archive_path":fmt.Sprintf("photos/%d_%s",p.ID,p.Storage)})}
    manifest["photos"]=photoMeta
    data,err:=json.MarshalIndent(manifest,"","  ")
    if err!=nil{fail(w,500,"构建导出清单失败");return}
    w.Header().Set("Content-Type","application/zip")
    w.Header().Set("Cache-Control","no-store")
    w.Header().Set("Content-Disposition",fmt.Sprintf("attachment; filename=%q","garden-diary-"+time.Now().Format("20060102")+".zip"))
    writer:=zip.NewWriter(w)
    defer writer.Close()
    out,err:=writer.Create("manifest.json")
    if err!=nil{return}
    if _,err:=out.Write(data);err!=nil{return}
    for _,p:=range photos {
        file,err:=os.Open(filepath.Join(a.cfg.DataDir,"originals",p.Storage))
        if err!=nil{continue}
        out,err:=writer.Create(fmt.Sprintf("photos/%d_%s",p.ID,p.Storage))
        if err==nil{_,err=io.Copy(out,file)}
        file.Close()
        if err!=nil{return}
    }
}
