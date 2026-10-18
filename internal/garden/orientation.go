package garden

import (
    "bytes"
    "encoding/binary"
    "image"
    "io"
    "os"
)

// jpegOrientation reads the small EXIF IFD0 orientation tag without trusting
// metadata-provided offsets as file paths or allocating image-sized buffers.
func jpegOrientation(path string) int {
    f,err:=os.Open(path)
    if err!=nil{return 1}
    defer f.Close()
    var start [2]byte
    if _,err:=io.ReadFull(f,start[:]);err!=nil||start[0]!=0xff||start[1]!=0xd8{return 1}
    for {
        var marker [2]byte
        if _,err:=io.ReadFull(f,marker[:]);err!=nil||marker[0]!=0xff{return 1}
        for marker[1]==0xff {
            if _,err:=io.ReadFull(f,marker[1:]);err!=nil{return 1}
        }
        if marker[1]==0xda||marker[1]==0xd9{return 1}
        if marker[1]==0xd8||marker[1]==0x01||(marker[1]>=0xd0&&marker[1]<=0xd7){continue}
        var length [2]byte
        if _,err:=io.ReadFull(f,length[:]);err!=nil{return 1}
        n:=int(binary.BigEndian.Uint16(length[:]))-2
        if n<0{return 1}
        if marker[1]==0xe1 {
            payload:=make([]byte,n)
            if _,err:=io.ReadFull(f,payload);err!=nil{return 1}
            if !bytes.HasPrefix(payload,[]byte("Exif\x00\x00")){continue}
            tiff:=payload[6:]
            if len(tiff)<8{return 1}
            var endian binary.ByteOrder
            switch string(tiff[:2]){
            case "II":endian=binary.LittleEndian
            case "MM":endian=binary.BigEndian
            default:return 1
            }
            if endian.Uint16(tiff[2:4])!=42{return 1}
            offset:=int(endian.Uint32(tiff[4:8]))
            if offset<0||offset+2>len(tiff){return 1}
            count:=int(endian.Uint16(tiff[offset:offset+2]))
            begin:=offset+2
            for i:=0;i<count&&i<128;i++{
                pos:=begin+i*12
                if pos+12>len(tiff){return 1}
                tag:=tiff[pos:pos+12]
                if endian.Uint16(tag[0:2])==0x0112 && endian.Uint16(tag[2:4])==3 && endian.Uint32(tag[4:8])==1 {
                    orientation:=int(endian.Uint16(tag[8:10]))
                    if orientation>=1&&orientation<=8{return orientation}
                }
            }
            return 1
        }
        if _,err:=f.Seek(int64(n),io.SeekCurrent);err!=nil{return 1}
    }
}
func applyOrientation(src image.Image,orientation int) image.Image {
    if orientation<2||orientation>8{return src}
    bounds:=src.Bounds();w,h:=bounds.Dx(),bounds.Dy()
    dw,dh:=w,h
    if orientation>=5{dw,dh=h,w}
    out:=image.NewNRGBA(image.Rect(0,0,dw,dh))
    for sy:=0;sy<h;sy++{
        for sx:=0;sx<w;sx++{
            x,y:=sx,sy
            switch orientation{
            case 2:x=w-1-sx
            case 3:x,y=w-1-sx,h-1-sy
            case 4:y=h-1-sy
            case 5:x,y=sy,sx
            case 6:x,y=h-1-sy,sx
            case 7:x,y=h-1-sy,w-1-sx
            case 8:x,y=sy,w-1-sx
            }
            out.Set(x,y,src.At(bounds.Min.X+sx,bounds.Min.Y+sy))
        }
    }
    return out
}
