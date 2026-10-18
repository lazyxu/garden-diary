package garden

import (
    "net/http"
    "time"
)
type dayRecord struct {
    EntryID int64 `json:"entry_id"`
    PlantID int64 `json:"plant_id"`
    PlantName string `json:"plant_name"`
    PhotoID int64 `json:"photo_id"`
    ObservedAt string `json:"observed_at"`
    Note string `json:"note"`
}
func (a *App) calendarDay(w http.ResponseWriter,r *http.Request,_ Session) {
    if !method(w,r,"GET"){return}
    day:=r.URL.Query().Get("date")
    if _,err:=time.Parse("2006-01-02",day);err!=nil{fail(w,400,"date 应为 YYYY-MM-DD");return}
    rows,err:=a.db.Query("SELECT e.id,e.plant_id,p.name,COALESCE((SELECT ph.id FROM photos ph WHERE ph.entry_id=e.id ORDER BY ph.id LIMIT 1),0),e.observed_at,e.note FROM entries e JOIN plants p ON p.id=e.plant_id WHERE substr(e.observed_at,1,10)=? ORDER BY e.observed_unix DESC,e.id DESC",day)
    if err!=nil{fail(w,500,"读取当天记录失败");return}
    defer rows.Close()
    result:=[]dayRecord{}
    for rows.Next(){
        var record dayRecord
        if err:=rows.Scan(&record.EntryID,&record.PlantID,&record.PlantName,&record.PhotoID,&record.ObservedAt,&record.Note);err!=nil{fail(w,500,"读取当天记录失败");return}
        result=append(result,record)
    }
    if rows.Err()!=nil{fail(w,500,"读取当天记录失败");return}
    jsonResponse(w,200,result)
}
