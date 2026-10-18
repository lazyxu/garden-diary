package garden

import (
    "bytes"
    "context"
    "errors"
    "fmt"
    "image"
    "image/jpeg"
    _ "image/png"
    "io"
    "log"
    "math"
    "mime"
    "mime/multipart"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"
    "time"

    "golang.org/x/image/draw"
)
type fileNames struct{Original,Thumb string}
type storedPhoto struct{
    StorageName string
    ThumbName string
    OriginalName string
    Mime string
    Size int64
}
const maxPhotoBytes=25<<20
func identify(data []byte)(string,string,error){
    if len(data)>=3 && bytes.Equal(data[:3],[]byte{0xff,0xd8,0xff}){return "image/jpeg",".jpg",nil}
    if len(data)>=8 && bytes.Equal(data[:8],[]byte{137,80,78,71,13,10,26,10}){return "image/png",".png",nil}
    if len(data)>=12 && bytes.Equal(data[4:8],[]byte("ftyp")) {
        brand:=string(data[8:12])
        switch brand {
        case "heic","heix","hevc","hevx","heim","heis","mif1","msf1":
            return "image/heic",".heic",nil
        }
    }
    return "","",errors.New("只支持 JPEG、PNG 和 HEIC 图片")
}
func(a *App) storeUpload(h *multipart.FileHeader)(p storedPhoto,err error){
    if h.Size>maxPhotoBytes{return p,fmt.Errorf("单张照片不能超过 %d MiB",maxPhotoBytes>>20)}
    source,err:=h.Open();if err!=nil{return p,err}
    defer source.Close()
    tmp,err:=os.CreateTemp(filepath.Join(a.cfg.DataDir,"tmp"),"upload-*")
    if err!=nil{return p,err}
    tmpPath:=tmp.Name()
    defer os.Remove(tmpPath)
    n,copyErr:=io.Copy(tmp,io.LimitReader(source,maxPhotoBytes+1))
    if err:=tmp.Close();err!=nil{return p,err}
    if copyErr!=nil{return p,copyErr}
    if n>maxPhotoBytes{return p,errors.New("单张照片过大")}
    f,err:=os.Open(tmpPath);if err!=nil{return p,err}
    header:=make([]byte,32)
    count,_:=io.ReadFull(f,header)
    f.Close()
    kind,ext,err:=identify(header[:count]);if err!=nil{return p,err}
    if kind!="image/heic" {
        f,err=os.Open(tmpPath);if err!=nil{return p,err}
        info,_,configErr:=image.DecodeConfig(f);f.Close()
        if configErr!=nil{return p,errors.New("图片无法解码")}
        if info.Width<=0||info.Height<=0||int64(info.Width)*int64(info.Height)>60_000_000{return p,errors.New("图片分辨率超出限制")}
    }
    token,err:=randomToken();if err!=nil{return p,err}
    storageName:=token+ext
    dest:=filepath.Join(a.cfg.DataDir,"originals",storageName)
    if err:=os.Rename(tmpPath,dest);err!=nil{return p,err}
    p=storedPhoto{StorageName:storageName,OriginalName:safeFilename(h.Filename),Mime:kind,Size:n}
    thumbName:=token+".jpg"
    if err:=a.buildThumbnail(dest,kind,filepath.Join(a.cfg.DataDir,"thumbs",thumbName));err!=nil{
        // Preserve the original even if an unsupported HEIC variant cannot be converted.
        log.Printf("thumbnail %s: %v",storageName,err)
    }else{p.ThumbName=thumbName}
    return p,nil
}
func safeFilename(raw string) string{
    name:=filepath.Base(strings.ReplaceAll(strings.ReplaceAll(raw,"\\","/"),"\r",""))
    name=strings.ReplaceAll(name,"\n","")
    if name==""||name=="."||name=="/" {return "photo"}
    if len(name)>180 {name=name[:180]}
    return name
}
func(a *App) buildThumbnail(original,kind,dest string) error {
    decodePath:=original
    if kind=="image/heic" {
        ctx,cancel:=context.WithTimeout(context.Background(),20*time.Second)
        defer cancel()
        temp,err:=os.CreateTemp(filepath.Join(a.cfg.DataDir,"tmp"),"converted-*.jpg")
        if err!=nil{return err}
        decodePath=temp.Name();temp.Close();defer os.Remove(decodePath)
        cmd:=exec.CommandContext(ctx,"heif-convert","-q","78",original,decodePath)
        if output,err:=cmd.CombinedOutput();err!=nil{return fmt.Errorf("HEIC 转换失败: %w (%s)",err,string(output))}
    }
    f,err:=os.Open(decodePath);if err!=nil{return err}
    config,_,err:=image.DecodeConfig(f);f.Close()
    if err!=nil{return err}
    if config.Width<1||config.Height<1||int64(config.Width)*int64(config.Height)>60_000_000{return errors.New("图片分辨率超过限制")}
    f,err=os.Open(decodePath);if err!=nil{return err}
    src,_,err:=image.Decode(f);f.Close()
    if err!=nil{return err}

    if kind=="image/jpeg" {src=applyOrientation(src,jpegOrientation(original))}
    if err:=saveScaledJPEG(src,560,dest);err!=nil{return err}
    if kind=="image/heic" {
        preview:=filepath.Join(a.cfg.DataDir,"previews",filepath.Base(dest))
        if err:=saveScaledJPEG(src,2048,preview);err!=nil{log.Printf("HEIC preview: %v",err)}
    }
    return nil
}
func saveScaledJPEG(src image.Image,maxSize float64,dest string) error {
    width,height:=src.Bounds().Dx(),src.Bounds().Dy()
    factor:=math.Min(1,maxSize/math.Max(float64(width),float64(height)))
    outW:=int(math.Max(1,math.Round(float64(width)*factor)))
    outH:=int(math.Max(1,math.Round(float64(height)*factor)))
    dst:=image.NewRGBA(image.Rect(0,0,outW,outH))
    draw.CatmullRom.Scale(dst,dst.Bounds(),src,src.Bounds(),draw.Over,nil)
    tmp,err:=os.CreateTemp(filepath.Dir(dest),".scaled-*")
    if err!=nil{return err}
    tempName:=tmp.Name()
    defer os.Remove(tempName)
    if err:=jpeg.Encode(tmp,dst,&jpeg.Options{Quality:83});err!=nil{tmp.Close();return err}
    if err:=tmp.Close();err!=nil{return err}
    return os.Rename(tempName,dest)
}
func(a *App) filesForEntry(id int64)([]fileNames,error){
    return a.photoFiles("SELECT storage_name,thumb_name FROM photos WHERE entry_id=?",id)
}
func(a *App) filesForPlant(id int64)([]fileNames,error){
    return a.photoFiles("SELECT ph.storage_name,ph.thumb_name FROM photos ph JOIN entries e ON e.id=ph.entry_id WHERE e.plant_id=?",id)
}
func(a *App) photoFiles(query string,id int64)([]fileNames,error){
    rows,err:=a.db.Query(query,id);if err!=nil{return nil,err}
    defer rows.Close()
    result:=[]fileNames{}
    for rows.Next(){
        var p fileNames
        if err:=rows.Scan(&p.Original,&p.Thumb);err!=nil{return nil,err}
        result=append(result,p)
    }
    return result,rows.Err()
}
func(a *App) removeFiles(files []fileNames){
    for _,p:=range files{
        if p.Original!=""{
            if err:=os.Remove(filepath.Join(a.cfg.DataDir,"originals",p.Original));err!=nil&&!os.IsNotExist(err){log.Printf("remove original: %v",err)}
        }
        if p.Thumb!=""{
            if err:=os.Remove(filepath.Join(a.cfg.DataDir,"thumbs",p.Thumb));err!=nil&&!os.IsNotExist(err){log.Printf("remove thumb: %v",err)}
            if err:=os.Remove(filepath.Join(a.cfg.DataDir,"previews",p.Thumb));err!=nil&&!os.IsNotExist(err){log.Printf("remove preview: %v",err)}
        }
    }
}
func(a *App) photoFile(w http.ResponseWriter,r *http.Request,_ Session){
    if !method(w,r,"GET"){return}
    segments:=strings.Split(strings.TrimPrefix(r.URL.Path,"/api/v1/photos/"),"/")
    if len(segments)!=2||(segments[1]!="original"&&segments[1]!="thumb"&&segments[1]!="preview"){fail(w,404,"照片不存在");return}
    id,err:=strconv.ParseInt(segments[0],10,64)
    if err!=nil||id<1{fail(w,404,"照片不存在");return}
    var original,thumbnail,filename,kind string
    err=a.db.QueryRow("SELECT storage_name,thumb_name,original_name,mime FROM photos WHERE id=?",id).Scan(&original,&thumbnail,&filename,&kind)
    if err!=nil{fail(w,404,"照片不存在");return}
    name:=original
    sub:="originals"
    if segments[1]=="thumb"&&thumbnail!="" {name=thumbnail;kind="image/jpeg";sub="thumbs"}
    if segments[1]=="preview"&&kind=="image/heic"&&thumbnail!="" {
        name=thumbnail;kind="image/jpeg";sub="previews"
        if _,err:=os.Stat(filepath.Join(a.cfg.DataDir,sub,name));err!=nil{sub="thumbs"}
    }
    if name==""||filepath.Base(name)!=name{fail(w,404,"照片不存在");return}
    path:=filepath.Join(a.cfg.DataDir,sub,name)
    file,err:=os.Open(path);if err!=nil{fail(w,404,"照片文件不存在");return}
    defer file.Close()
    stat,err:=file.Stat();if err!=nil||stat.IsDir(){fail(w,404,"照片文件不存在");return}
    w.Header().Set("Content-Type",kind)
    w.Header().Set("Cache-Control","private, no-store")
    if r.URL.Query().Get("download")=="1" {
        w.Header().Set("Content-Disposition",mime.FormatMediaType("attachment",map[string]string{"filename":filename}))
    } else {w.Header().Set("Content-Disposition","inline")}
    http.ServeContent(w,r,filename,stat.ModTime(),file)
}
