package garden

import (
    "bytes"
    "image"
    "image/color"
    "image/jpeg"
    "os"
    "path/filepath"
    "testing"
)
func TestJPEGEXIFOrientation(t *testing.T){
    src:=image.NewNRGBA(image.Rect(0,0,2,3))
    src.Set(0,0,color.NRGBA{R:255,A:255})
    src.Set(1,2,color.NRGBA{G:255,A:255})
    var jpg bytes.Buffer
    if err:=jpeg.Encode(&jpg,src,&jpeg.Options{Quality:90});err!=nil{t.Fatal(err)}
    tiff:=[]byte{'I','I',42,0,8,0,0,0,1,0,0x12,1,3,0,1,0,0,0,6,0,0,0,0,0,0,0}
    payload:=append([]byte{'E','x','i','f',0,0},tiff...)
    size:=len(payload)+2
    encoded:=append([]byte{},jpg.Bytes()[:2]...)
    encoded=append(encoded,0xff,0xe1,byte(size>>8),byte(size))
    encoded=append(encoded,payload...)
    encoded=append(encoded,jpg.Bytes()[2:]...)
    path:=filepath.Join(t.TempDir(),"photo.jpg")
    if err:=os.WriteFile(path,encoded,0600);err!=nil{t.Fatal(err)}
    if got:=jpegOrientation(path);got!=6{t.Fatalf("orientation=%d want 6",got)}
    rotated:=applyOrientation(src,6)
    if got:=rotated.Bounds().Dx();got!=3{t.Fatalf("width=%d want 3",got)}
    if got:=rotated.Bounds().Dy();got!=2{t.Fatalf("height=%d want 2",got)}
    if rotated.At(2,0)!=src.At(0,0){t.Fatal("rotation lost top-left pixel")}
    if rotated.At(0,1)!=src.At(1,2){t.Fatal("rotation lost bottom-right pixel")}
}
